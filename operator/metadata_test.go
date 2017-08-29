package operator

import (
	"fmt"
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

func TestGetMetadata(t *testing.T) {
	md, err := GetMetadata()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", *md)
}

func TestMetadata_ToConfigMapData(t *testing.T) {
	m := &Metadata{}
	if err := yaml.Unmarshal([]byte(initData), m); err != nil {
		t.Fatalf("unmarshal metadata error: %v", err)
	}
	mp, err := m.ToConfigMapData()
	if err != nil {
		t.Fatal(err)
	}
	for key, val := range mp {
		fmt.Printf("key: %s, val: %s\n", key, val)
	}
}

func TestConfigMapDataToMetadata(t *testing.T) {
	src := &Metadata{}
	if err := yaml.Unmarshal([]byte(initData), src); err != nil {
		t.Fatalf("unmarshal metadata error: %v", err)
	}
	mp, err := src.ToConfigMapData()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("map: %+v\n", mp)
	m, err := ConfigMapDataToMetadata(mp)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("metadata: %+v\n", m)
}
