package models

import (
	"encoding/json"

	yaml "gopkg.in/yaml.v2"

	"errors"
	"fmt"

	"strings"

	"sync/atomic"

	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/pkg/k8sutil"
	"github.com/ffan/tidb-k8s/pkg/storage"
)

var initData = `
versions:
  - rc2
  - latest
basic:
  pd:
    cpu: 500
    memory: 1024
    max: 7
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
  volume: ""
  proxys: ""
approvalConditions:
  kvReplicas: 3
  dbReplicas: 1
`

// Unit 共享单元
type Unit struct {
	CPU      int `json:"cpu"`
	Mem      int `json:"memory" yaml:"memory"`
	Capacity int `json:"capacity,omitempty"`
	Max      int `json:"max"`
}

// Units 包含tidb/tikv/pd三个模块的共享信息
type Units struct {
	Pd   Unit `json:"pd"`
	Tikv Unit `json:"tikv"`
	Tidb Unit `json:"tidb"`
}

// K8s kubernetes服务配置
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

// Metadata tidb metadata
type Metadata struct {
	Versions       []string           `json:"versions"`
	Units          Units              `json:"basic" yaml:"basic"`
	Specifications []Specification    `json:"specifications"`
	K8s            K8s                `json:"k8s"`
	AC             ApprovalConditions `json:"ac" yaml:"approvalConditions"`
}

const (
	syncMetadataInterval = 5 * time.Second
)

var (
	metaS storage.Storage

	count int32
	md    *Metadata
)

// Init Metadata
func metaInit() {
	s, err := storage.NewDefaultStorage(tableMetadata, etcdAddress)
	if err != nil {
		panic(fmt.Errorf("Create storage metadata error: %v", err))
	}
	metaS = s

	initMetadataIfNot()

	go func() {
		m, err := GetMetadata()
		if err != nil {
			logs.Critical("sync metadata error: %", err)
		}
		md = m
		time.Sleep(syncMetadataInterval)
	}()
}

func initMetadataIfNot() {
	if !forceInitMd {
		return
	}
	m := &Metadata{}
	if err := yaml.Unmarshal([]byte(initData), m); err != nil {
		panic(fmt.Sprintf("unmarshal metadata error: %v", err))
	}

	// get proxys ip

	sel := map[string]string{
		"node-role.proxy": "",
	}
	ps, err := k8sutil.GetNodesExternalIP(sel)
	if err != nil {
		panic(fmt.Sprintf("get proxys error: %v", err))
	}
	m.K8s.Proxys = strings.Join(ps, ",")

	logs.Info("%+v", m)
	m.Update()
	logs.Info("Init local metadata to storage")
}

// NewMetadata create a metadata instance
func NewMetadata() *Metadata {
	return &Metadata{}
}

// Create 持久化metadata
func (m *Metadata) Create() error {
	if err := m.adjust(); err != nil {
		return err
	}
	data, err := json.Marshal(m)
	if err != nil {
		return errors.New("marshal meta data err")
	}
	metaS.Create("", data)
	return nil
}

func (m *Metadata) adjust() (err error) {
	if m.Units.Pd.Max%2 == 0 {
		m.Units.Pd.Max = m.Units.Pd.Max + 1
	}
	return nil
}

// Update update metadata
func (m *Metadata) Update() error {
	if err := m.adjust(); err != nil {
		return err
	}
	data, err := json.Marshal(m)
	if err != nil {
		return errors.New("marshal meta data err")
	}
	metaS.Update("", data)
	return nil
}

// GetMetadata get metadata
func GetMetadata() (*Metadata, error) {
	data, err := metaS.Get("")
	if err != nil {
		return nil, err
	}
	m := Metadata{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	return &m, err
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
	ps := strings.Split(m.K8s.Proxys, ",")
	if len(ps) < 3 {
		return ps
	}
	hosts[0] = ps[int(atomic.AddInt32(&count, 1))%len(ps)]
	hosts[1] = ps[int(atomic.AddInt32(&count, 1))%len(ps)]
	return hosts
}
