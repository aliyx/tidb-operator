package operator

import (
	"testing"

	"fmt"

	"github.com/astaxie/beego/logs"
	yaml "gopkg.in/yaml.v2"
)

func Test_UnmarshalMetadata(t *testing.T) {
	m := &MetaSpec{}
	if err := yaml.Unmarshal([]byte(initData), m); err != nil {
		t.Errorf("unmarshal metadata error: %v", err)
	}
	logs.Debug("%+v", m)
}

func TestGetMetadata(t *testing.T) {
	md, err := GetMetadata()
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("%+v\n", *md)
}
