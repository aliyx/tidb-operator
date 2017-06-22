package storage

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/ffan/tidb-k8s/pkg/k8sutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	k8sAddr = "http://10.213.44.128:10218"
	s       *Storage
)

type testObj struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ObjectMeta `json:"metadata,omitempty"`
	Name            string            `json:"name,omitempty"`
}

func TestMain(m *testing.M) {
	k8sutil.Init(k8sAddr)
	st, err := NewStorage(k8sAddr, "default", "metadata")
	if err != nil {
		log.Fatalln("cannt create tpr client url=%s, %v", k8sAddr, err)
	}
	s = st
	os.Exit(m.Run())
}

func TestStorage_Get(t *testing.T) {
	test := &testObj{}
	err := s.Get("test", test)
	if err != nil {
		t.Errorf("%v", err)
	}
	fmt.Printf("%#v", test)
}

func TestStorage_Update(t *testing.T) {
	test := &testObj{}
	err := s.Get("test", test)
	if err != nil {
		t.Errorf("%v", err)
	}
	test.Name = "test"
	err = s.Update("test", test)
	if err != nil {
		t.Errorf("%v", err)
	}
}

func TestStorage_Create(t *testing.T) {
	test := &testObj{
		Metadata: metav1.ObjectMeta{
			Name: "test",
		},
	}
	if err := s.Create(test); err != nil {
		t.Errorf("%v", err)
	}
}
