package operator

import (
	"encoding/json"
	"flag"
	"reflect"

	yaml "gopkg.in/yaml.v2"

	"fmt"

	"strings"

	"sync/atomic"

	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/storage"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"

	"github.com/fatih/structs"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Unit container spec
type Unit struct {
	CPU      int `json:"cpu"`
	Mem      int `json:"memory" yaml:"memory"`
	Capacity int `json:"capacity,omitempty"`
	Max      int `json:"max"`
}

// K8sConfig kube config
type K8sConfig struct {
	HostPath string   `json:"hostPath"`
	Mount    string   `json:"mount"`
	Proxys   []string `json:"proxys"`
}

// Path real host path
func (k K8sConfig) Path() string {
	return k.HostPath + k.Mount
}

// AvailableVolume path eg: /mmt/data0, /mmt/data1...
func (k K8sConfig) AvailableVolume() bool {
	return len(k.Path()) > 2 && strings.HasPrefix(k.Path(), "/")
}

// ApprovalConditions Tikv and tidb more than the number of replicas of the conditions,
// you need admin approval
type ApprovalConditions struct {
	KvReplicas uint `json:"kvReplicas" yaml:"kvReplicas"`
	DbReplicas uint `json:"dbReplicas" yaml:"dbReplicas"`
}

// Replicas controller
type Replicas []int

func (r Replicas) getPd() int {
	return r[0]
}

func (r Replicas) getTikv() int {
	return r[1]
}

func (r Replicas) getTidb() int {
	return r[2]
}

// Preference database preferences
type Preference struct {
	Name     string   `json:"name"`
	Desc     string   `json:"desc"`
	Replicas Replicas `json:"replicas"`
}

// Specification 规格说明书，比如“1核2 GB ...”
type Specification struct {
	Name        string       `json:"name"`
	Desc        string       `json:"desc"`
	Preferences []Preference `json:"preferences" yaml:"preferences"`
}

// Metadata ...
type Metadata struct {
	Versions  []string  `json:"versions"`
	Pd        Unit      `json:"pd"`
	Tikv      Unit      `json:"tikv"`
	Tidb      Unit      `json:"tidb"`
	K8sConfig K8sConfig `json:"kubernetesConfig"`

	AC             ApprovalConditions `json:"approvalConditions" yaml:"approvalConditions"`
	Specifications []Specification    `json:"specifications"`
}

const (
	syncMetadataInterval = 10 * time.Second

	defaultMetadatConfigName = "tidb-metadata"
)

var (
	hostPath        string
	mount           string
	defaultHostPath = "/mnt"
	defaultMount    = "data"

	count int32

	// for test
	waitProxys = true
)

func init() {
	flag.StringVar(&hostPath, "host-path", defaultHostPath, "The tikv hostPath volume.")
	flag.StringVar(&mount, "mount", defaultMount, "The path prefix of tikv mount.")
}

// ToConfigMapData tranfer metadata to config map
func (m *Metadata) ToConfigMapData() (map[string]string, error) {
	structs.DefaultTagName = "json"
	temp := structs.Map(m)
	mp := map[string]string{}
	for key, obj := range temp {
		b, err := json.Marshal(obj)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshal config for class %v: %v", key, err)
		}
		mp[key] = string(b)
	}
	return mp, nil
}

// ConfigMapDataToMetadata convert config data to metadata obj
func ConfigMapDataToMetadata(mp map[string]string) (*Metadata, error) {
	md := NewMetadata()
	for class, val := range mp {
		switch class {
		case "approvalConditions":
			ac := ApprovalConditions{}
			if err := json.Unmarshal([]byte(val), &ac); err != nil {
				return nil, err
			}
			md.AC = ac
		case "kubernetesConfig":
			kc := K8sConfig{}
			if err := json.Unmarshal([]byte(val), &kc); err != nil {
				return nil, err
			}
			md.K8sConfig = kc
		case "pd":
			u := Unit{}
			if err := json.Unmarshal([]byte(val), &u); err != nil {
				return nil, err
			}
			md.Pd = u
		case "tikv":
			u := Unit{}
			if err := json.Unmarshal([]byte(val), &u); err != nil {
				return nil, err
			}
			md.Tikv = u
		case "tidb":
			u := Unit{}
			if err := json.Unmarshal([]byte(val), &u); err != nil {
				return nil, err
			}
			md.Tidb = u
		case "specifications":
			ss := []Specification{}
			if err := json.Unmarshal([]byte(val), &ss); err != nil {
				return nil, err
			}
			md.Specifications = ss
		case "versions":
			vs := []string{}
			if err := json.Unmarshal([]byte(val), &vs); err != nil {
				return nil, err
			}
			md.Versions = vs
		}

	}
	return md, nil
}

// Init Metadata
func metaInit() {
	go func() {
		for {
			time.Sleep(syncMetadataInterval)
			m, err := GetMetadata()
			if err != nil {
				if err != storage.ErrNoNode {
					logs.Error("could get metadata error: %v", err)
				}
				continue
			}

			// sync proxys
			ps, _ := k8sutil.GetNodesExternalIP(map[string]string{
				"node-role.proxy": "",
			})
			if len(ps) > 0 && !reflect.DeepEqual(ps, m.K8sConfig.Proxys) {
				logs.Info("cluster proxy has changed, from %v to %v", m.K8sConfig.Proxys, ps)
				m.K8sConfig.Proxys = ps
				if err = m.Update(); err != nil {
					logs.Error("failed to update metadata: %v", err)
				}
			}
		}
	}()

	initMetadataIfNot()
}

func initMetadataIfNot() {
	var err error
	md, _ := GetMetadata()
	defer func() {
		if err != nil || !waitProxys {
			return
		}
		for {
			if len(md.K8sConfig.Proxys) == 0 {
				logs.Warning("waiting for labeling the proxy's node")
				time.Sleep(3 * time.Second)
				md, _ = GetMetadata()
				continue
			}
			return
		}
	}()
	if md != nil {
		if !forceInitMd {
			return
		}
	}
	md = &Metadata{}
	if err = yaml.Unmarshal([]byte(initData), md); err != nil {
		panic(fmt.Sprintf("unmarshal metadata error: %v", err))
	}

	// get proxys ip
	ps, err := k8sutil.GetNodesExternalIP(map[string]string{
		"node-role.proxy": "",
	})
	if err != nil {
		panic(fmt.Sprintf("could not get proxys: %v", err))
	}

	md.K8sConfig = K8sConfig{
		HostPath: hostPath,
		Mount:    mount,
		Proxys:   ps,
	}
	if !md.K8sConfig.AvailableVolume() {
		panic(fmt.Sprintf("please specify PV hostpath and mount, hostPath: %q, mount:%q",
			md.K8sConfig.HostPath, md.K8sConfig.Mount))
	}

	logs.Info("%+v", md)
	if err = md.createOrUpdate(); err != nil {
		panic(fmt.Sprintf("init metadata error: %v", err))
	}
	logs.Info("Init local metadata to storage")
}

// NewMetadata create a metadata instance
func NewMetadata() *Metadata {
	return &Metadata{}
}

// createOrUpdate create if no exist, else update
func (m *Metadata) createOrUpdate() (err error) {
	if err = m.adjust(); err != nil {
		return
	}
	cm, err := m.ToConfigMapData()
	if err != nil {
		return err
	}
	_, err = k8sutil.GetConfigmap(defaultMetadatConfigName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err = k8sutil.CreateConfigmap(defaultMetadatConfigName, cm); err != nil {
				return err
			}
			return nil
		}
		return err
	}
	if err = k8sutil.UpdateConfigMap(defaultMetadatConfigName, cm); err != nil {
		return err
	}
	return nil
}

// Update update metadata
func (m *Metadata) Update() (err error) {
	if !m.K8sConfig.AvailableVolume() {
		return fmt.Errorf("please specify PV hostpath and mount, hostPath: %q, mount:%q",
			m.K8sConfig.HostPath, m.K8sConfig.Mount)
	}
	if len(m.K8sConfig.Proxys) < 1 {
		return fmt.Errorf("unavailable proxys: %v", m.K8sConfig.Proxys)
	}
	cm, err := m.ToConfigMapData()
	if err != nil {
		return err
	}
	return k8sutil.UpdateConfigMap(defaultMetadatConfigName, cm)
}

func (m *Metadata) adjust() (err error) {
	m.Pd.Max = 3
	return nil
}

// GetMetadata get metadata
func GetMetadata() (*Metadata, error) {
	cm, err := k8sutil.GetConfigmap(defaultMetadatConfigName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, storage.ErrNoNode
		}
		return nil, err
	}
	return ConfigMapDataToMetadata(cm)
}

func getNonNullMetadata() *Metadata {
	md, err := GetMetadata()
	if err != nil {
		panic("could not get metadata")
	}
	return md
}

func getNamespace() string {
	return k8sutil.Namespace
}

func getProxys() []string {
	hosts := make([]string, 2)
	m := getNonNullMetadata()
	ps := m.K8sConfig.Proxys
	if len(ps) < 3 {
		return ps
	}
	hosts[0] = ps[int(atomic.AddInt32(&count, 1))%len(ps)]
	hosts[1] = ps[int(atomic.AddInt32(&count, 1))%len(ps)]
	return hosts
}

var initData = `
versions:
- rc4
- latest
pd:
  cpu: 500
  memory: 1024
  max: 3
tikv:
  cpu: 500
  memory: 1024
  capacity: 100
  max: 10
tidb:
  cpu: 500
  memory: 1024
  max: 10
approvalConditions:
  kvReplicas: 3
  dbReplicas: 2
specifications:
- name: "2核 4GB"
  desc: ""
  preferences:
  - name: "普通"
    replicas: 
    - 3
    - 3
    - 2
- name: "4核 8GB"
  desc: ""
  preferences:
  - name: "普通"
    desc: "连接数:100 IOPS:1000"
    replicas: 
    - 3
    - 4
    - 5
  - name: "存储"
    desc: "连接数:100 存储：200GB"
    replicas: 
    - 3
    - 6
    - 3
  - name: "计算"
    desc: "连接数:100 存储：100GB"
    replicas: 
    - 3
    - 3
    - 5
`
