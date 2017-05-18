package models

import (
	"encoding/json"
	"math/rand"
	"time"

	yaml "gopkg.in/yaml.v2"

	"errors"
	"fmt"

	"strings"

	"sync/atomic"

	"github.com/astaxie/beego/logs"
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
  namespace: default
  registry: "10.209.224.13:10500/ffan/rds"
  volume: ""
  proxys: "10.213.44.128,10.213.129.73,10.213.129.74"
`

var (
	// metaS all metadata model共享storage
	metaS Storage
	// count 原子计数器
	count int32
)

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
	Namespace string `json:"namespace"`
	Registry  string `json:"registry"`
	Volume    string `json:"volume"`
	Proxys    string `json:"proxys"`
}

// Metadata tidb元数据
type Metadata struct {
	Versions       []string        `json:"versions"`
	Units          Units           `json:"basic" yaml:"basic"`
	Specifications []Specification `json:"specifications"`
	K8s            K8s             `json:"k8s"`
}

// Init Metadata model初始化
func metaInit() {
	rand.Seed(time.Now().Unix())
	s, err := newStorage(metaNamespace)
	if err != nil {
		panic(fmt.Errorf("Create storage metadata error: %v", err))
	}
	metaS = s

	initMetadataIfNot()
}

func initMetadataIfNot() {
	if !forceInitMd {
		return
	}
	m := &Metadata{}
	if err := yaml.Unmarshal([]byte(initData), m); err != nil {
		panic(fmt.Sprintf("unmarshal metadata error: %v", err))
	}
	logs.Debug("%+v", m)
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

// GetMetadata 获取Metadata
func GetMetadata() (*Metadata, error) {
	if metaS.IsNil() {
		return nil, fmt.Errorf("metaS not created")
	}
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

func getNamespace() string {
	m, err := GetMetadata()
	if err != nil {
		return "default"
	}
	return m.K8s.Namespace
}

func getProxys() []string {
	hosts := make([]string, 2)
	m, err := GetMetadata()
	if err != nil {
		logs.Error("Cannt get metadata")
		return hosts
	}
	ps := strings.Split(m.K8s.Proxys, ",")
	if len(ps) < 3 {
		return ps
	}
	hosts[0] = ps[int(atomic.AddInt32(&count, 1))%len(ps)]
	hosts[1] = ps[int(atomic.AddInt32(&count, 1))%len(ps)]
	return hosts
}
