package operator

import (
	"flag"
	"reflect"

	yaml "gopkg.in/yaml.v2"

	"fmt"

	"strings"

	"sync/atomic"

	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/spec"
	"github.com/ffan/tidb-operator/pkg/storage"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var initData = `
versions:
- rc3
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

var (
	hostPath string
	mount    string

	defaultHostPath = "/"
	defaultMount    = "data"
)

func init() {
	flag.StringVar(&hostPath, "host-path", defaultHostPath, "The tikv hostPath volume.")
	flag.StringVar(&mount, "mount", defaultMount, "The path prefix of tikv mount.")
}

// Unit container spec
type Unit struct {
	CPU      int `json:"cpu"`
	Mem      int `json:"memory" yaml:"memory"`
	Capacity int `json:"capacity,omitempty"`
	Max      int `json:"max"`
}

// K8s config
type K8s struct {
	HostPath string   `json:"hostPath" yaml:"hostPath"`
	Mount    string   `json:"mount" yaml:"mount"`
	Proxys   []string `json:"proxys"`
}

// Path real host path
func (k K8s) Path() string {
	return k.HostPath + k.Mount
}

// AvailableVolume path eg: /mmt/data0, /mmt/data1...
func (k K8s) AvailableVolume() bool {
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

// MetaSpec tidb metadata
type MetaSpec struct {
	Versions []string `json:"versions"`
	Pd       Unit     `json:"pd"`
	Tikv     Unit     `json:"tikv"`
	Tidb     Unit     `json:"tidb"`
	K8s      K8s      `json:"kubernetesConfig"`

	AC             ApprovalConditions `json:"approvalConditions" yaml:"approvalConditions"`
	Specifications []*Specification   `json:"specifications"`
}

// Metadata resource
type Metadata struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec *MetaSpec `json:"spec"`
}

const (
	syncMetadataInterval = 10 * time.Second
)

var (
	metaS *storage.Storage

	count int32
	md    *Metadata
)

// Init Metadata
func metaInit() {
	s, err := storage.NewStorage(getNamespace(), spec.TPRKindMetadata)
	if err != nil {
		panic(fmt.Errorf("Create storage metadata error: %v", err))
	}
	metaS = s

	initMetadataIfNot()

	go func() {
		for {
			time.Sleep(syncMetadataInterval)
			m, err := GetMetadata()
			if err != nil {
				logs.Critical("sync metadata  cache error: %v", err)
				continue
			}
			if m != nil {
				continue
			}

			// sync proxys
			ps, _ := k8sutil.GetNodesExternalIP(map[string]string{
				"node-role.proxy": "",
			})
			if len(ps) > 0 && !reflect.DeepEqual(ps, m.Spec.K8s.Proxys) {
				logs.Warn("cluster proxy has changed, from %v to %v")
				m.Spec.K8s.Proxys = ps
				if err = m.Update(); err != nil {
					logs.Error("failed to update metadata: %v", err)
				}
			}
		}
	}()
}

func initMetadataIfNot() {
	m, _ := GetMetadata()
	if m != nil {
		if !forceInitMd {
			return
		}
	}
	ms := &MetaSpec{}
	if err := yaml.Unmarshal([]byte(initData), ms); err != nil {
		panic(fmt.Sprintf("unmarshal metadata error: %v", err))
	}

	// get proxys ip
	ps, err := k8sutil.GetNodesExternalIP(map[string]string{
		"node-role.proxy": "",
	})
	if err != nil {
		panic(fmt.Sprintf("could not get proxys: %v", err))
	}

	ms.K8s = K8s{
		HostPath: hostPath,
		Mount:    mount,
		Proxys:   ps,
	}
	if !ms.K8s.AvailableVolume() {
		panic(fmt.Sprintf("please specify PV hostpath and mount, hostPath: %q, mount:%q",
			ms.K8s.HostPath, ms.K8s.Mount))
	}

	m = &Metadata{
		TypeMeta: metav1.TypeMeta{
			Kind:       spec.TPRKindMetadata,
			APIVersion: spec.APIVersion,
		},
		Metadata: metav1.ObjectMeta{
			Name: "metadata",
		},
		Spec: ms,
	}
	logs.Info("%+v", m.Spec)
	if err = m.CreateOrUpdate(); err != nil {
		panic(fmt.Sprintf("init metadata error: %v", err))
	}
	logs.Info("Init local metadata to storage")
}

// NewMetadata create a metadata instance
func NewMetadata() *Metadata {
	return &Metadata{}
}

// CreateOrUpdate create if no exist, else update
func (m *Metadata) CreateOrUpdate() (err error) {
	if err = m.adjust(); err != nil {
		return
	}
	err = metaS.Delete(m.Metadata.Name)
	if err != nil && err != storage.ErrNoNode {
		return
	}
	return metaS.Create(m)
}

// Update update metadata
func (m *Metadata) Update() (err error) {
	if !m.Spec.K8s.AvailableVolume() {
		return fmt.Errorf("please specify PV hostpath and mount, hostPath: %q, mount:%q",
			m.Spec.K8s.HostPath, m.Spec.K8s.Mount)
	}
	if len(m.Spec.K8s.Proxys) < 1 {
		return fmt.Errorf("unavailable proxys: %v", m.Spec.K8s.Proxys)
	}
	return metaS.RetryUpdate(m.Metadata.Name, m)
}

func (m *Metadata) adjust() (err error) {
	m.Spec.Pd.Max = 3
	return nil
}

// GetMetadata get metadata
func GetMetadata() (*Metadata, error) {
	m := NewMetadata()
	err := metaS.Get("metadata", m)
	if err != nil {
		return nil, err
	}
	return m, nil
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
	ps := m.Spec.K8s.Proxys
	if len(ps) < 3 {
		return ps
	}
	hosts[0] = ps[int(atomic.AddInt32(&count, 1))%len(ps)]
	hosts[1] = ps[int(atomic.AddInt32(&count, 1))%len(ps)]
	return hosts
}
