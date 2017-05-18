package models

import (
	"testing"

	"github.com/astaxie/beego/logs"
	yaml "gopkg.in/yaml.v2"
)

func Test_UnmarshalMetadata(t *testing.T) {
	m := &Metadata{}
	if err := yaml.Unmarshal([]byte(initData), m); err != nil {
		t.Errorf("unmarshal metadata error: %v", err)
	}
	logs.Debug("%+v", m)
}
