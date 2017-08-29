package k8sutil

import (
	"encoding/json"
	"time"

	"k8s.io/api/extensions/v1beta1"
)

// CreateDaemonsetByJSON ...
func CreateDaemonsetByJSON(
	j []byte,
	timeout time.Duration,
	updateFunc func(*v1beta1.DaemonSet)) (*v1beta1.DaemonSet, error) {
	ds := &v1beta1.DaemonSet{}
	if err := json.Unmarshal(j, ds); err != nil {
		return nil, err
	}
	updateFunc(ds)
	return CreateDaemonset(ds, timeout)
}

// CreateDaemonset ...
func CreateDaemonset(ds *v1beta1.DaemonSet, timeout time.Duration) (*v1beta1.DaemonSet, error) {
	return kubecli.ExtensionsV1beta1().DaemonSets(Namespace).Create(ds)
}
