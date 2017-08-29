package k8sutil

import (
	"github.com/astaxie/beego/logs"
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createNamespace(name string) error {
	ns := &v1.Namespace{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: name,
		},
	}
	retNs, err := kubecli.CoreV1().Namespaces().Create(ns)
	if err != nil {
		return err
	}
	logs.Info(`Namespace "%s" created`, retNs.GetName())
	return nil
}

func delNamespace(name string) error {
	if err := kubecli.CoreV1().Namespaces().Delete(name, meta_v1.NewDeleteOptions(0)); err != nil {
		return err
	}
	logs.Warn(`Namespace "%s" deleted`, name)
	return nil
}
