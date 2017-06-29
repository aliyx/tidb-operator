package operator

import (
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
- rc2
- rc3
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
k8s:
  volume: "/data"
  proxys: ""
approvalConditions:
  kvReplicas: 3
  dbReplicas: 1
specifications:
- name: "2核 4GB"
  desc: ""
  preferences:
  - name: "普通"
    replicas: 
    - 3
    - 3
    - 1
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

// Unit container spec
type Unit struct {
	CPU      int `json:"cpu"`
	Mem      int `json:"memory" yaml:"memory"`
	Capacity int `json:"capacity,omitempty"`
	Max      int `json:"max"`
}

// K8s config
type K8s struct {
	Volume string `json:"volume"`
	Proxys string `json:"proxys"`
}

// ApprovalConditions Tikv and tidb more than the number of replicas of the conditions,
// you need admin approval
type ApprovalConditions struct {
	KvReplicas uint `json:"kvr" yaml:"kvReplicas"`
	DbReplicas uint `json:"dbr" yaml:"dbReplicas"`
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

// Preference 数据库偏好
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
	K8s      K8s      `json:"k8s"`

	AC             ApprovalConditions `json:"ac" yaml:"approvalConditions"`
	Specifications []*Specification   `json:"specifications"`
}

// Metadata resource
type Metadata struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec *MetaSpec `json:"spec"`
}

const (
	syncMetadataInterval = 5 * time.Second
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
			m, err := GetMetadata()
			if err != nil {
				logs.Critical("sync metadata error: %", err)
			}
			md = m
			if md.Spec.K8s.Volume == "" || md.Spec.K8s.Volume == "/tmp" {
				logs.Warn("Please specify PV hostpath")
			}
			time.Sleep(syncMetadataInterval)
		}
	}()
}

func initMetadataIfNot() {
	if !forceInitMd {
		return
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
		panic(fmt.Sprintf("get proxys error: %v", err))
	}
	ms.K8s.Proxys = strings.Join(ps, ",")

	m := &Metadata{
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
		return err
	}
	tmp := NewMetadata()
	if err = metaS.Get(m.Metadata.Name, tmp); err == storage.ErrNoNode {
		return metaS.Create(m)
	}
	return metaS.Update(m.Metadata.Name, m)
}

// Update update metadata
func (m *Metadata) Update() (err error) {
	return metaS.Update(m.Metadata.Name, m)
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

func getCachedMetadata() *Metadata {
	return md
}

func getNamespace() string {
	return k8sutil.Namespace
}

func getProxys() []string {
	hosts := make([]string, 2)
	m := getCachedMetadata()
	ps := strings.Split(m.Spec.K8s.Proxys, ",")
	if len(ps) < 3 {
		return ps
	}
	hosts[0] = ps[int(atomic.AddInt32(&count, 1))%len(ps)]
	hosts[1] = ps[int(atomic.AddInt32(&count, 1))%len(ps)]
	return hosts
}
