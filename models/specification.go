package models

import (
	"encoding/json"
	"fmt"

	yaml "gopkg.in/yaml.v2"

	"github.com/astaxie/beego/logs"
)

// initSpecs 初始测试数据
var initSpecs = `
- name: "2核 4GB"
  desc: ""
  preferences:
  - name: "普通"
    pd:
      replicas: 3
    tikv:
      replicas: 3
    tidb:
      replicas: 1
- name: "4核 8GB"
  desc: ""
  preferences:
  - name: "普通"
    desc: "连接数:100 IOPS:1000"
    pd:
      replicas: 3
    tikv:
      replicas: 4
    tidb:
      replicas: 4
  - name: "存储"
    desc: "连接数:100 存储：200GB"
    pd:
      replicas: 3
    tikv:
      replicas: 6
    tidb:
      replicas: 2
  - name: "计算"
    desc: "连接数:100 存储：100GB"
    pd:
      replicas: 3
    tikv:
      replicas: 3
    tidb:
      replicas: 5
`

var (
	specKey = "specifications"
)

// Spec tidb每个模块的计算单元
type Spec struct {
	Replicas int `json:"replicas"`
}

// Preference 数据库偏好
type Preference struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
	Pd   Spec   `json:"pd"`
	Tikv Spec   `json:"tikv"`
	Tidb Spec   `json:"tidb"`
}

// Specification 规格说明书，比如“1核2 GB ...”
type Specification struct {
	Name        string       `json:"name"`
	Desc        string       `json:"desc"`
	Preferences []Preference `json:"preferences" yaml:"preferences"`
}

// Specifications sepc slice
type Specifications []Specification

// NewSpecifications new a instance
func NewSpecifications() *Specifications {
	return &Specifications{}
}

func specInit() {
	if !forceInitMd {
		return
	}
	specs := Specifications{}
	if err := yaml.Unmarshal([]byte(initSpecs), &specs); err != nil {
		panic(fmt.Sprintf("unmarshal specifications error: %v", err))
	}
	logs.Debug("%+v", specs)
	if err := specs.Create(); err != nil {
		logs.Error("Init specifications error: %v", err)
	}
	logs.Info("Init local specifications to storage")
}

// Create create specification
func (sp *Specifications) Create() error {
	data, err := json.Marshal(sp)
	if err != nil {
		return err
	}
	metaS.Create(specKey, data)
	return nil
}

// Update update specification
func (sp *Specifications) Update() error {
	data, err := json.Marshal(sp)
	if err != nil {
		return err
	}
	metaS.Update(specKey, data)
	return nil
}

// GetSpecs 获取all specification
func GetSpecs() (*Specifications, error) {
	data, err := metaS.Get(specKey)
	if err != nil {
		return nil, err
	}
	sp := Specifications{}
	if err := json.Unmarshal(data, &sp); err != nil {
		return nil, err
	}

	return &sp, err
}
