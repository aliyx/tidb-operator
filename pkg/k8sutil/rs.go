package k8sutil

import (
	"fmt"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/pkg/httputil"
	"github.com/ghodss/yaml"
	"github.com/tidwall/gjson"
)

func ScaleReplicationcontroller(name string, replicas int) error {
	k8sMu.Lock()
	defer k8sMu.Unlock()
	if replicas < 0 {
		replicas = 0
	}
	pt := []byte(fmt.Sprintf(rcPatch, replicas))
	err := httputil.Patch(fmt.Sprintf(k8sRcURL+"/%s/scale", masterHost, Namespace, name), pt, defaultk8sReqTimeout)
	if err != nil {
		return err
	}
	logs.Info(`Scale "%s" replicationcontroller: %d`, name, replicas)
	return nil
}

func CreateRs(s string) error {
	k8sMu.Lock()
	defer k8sMu.Unlock()
	logs.Info("yaml: %s", s)
	url := fmt.Sprintf(k8sRsURL, masterHost, Namespace)
	j, _ := yaml.YAMLToJSON([]byte(s))
	resp, err := httputil.Post(url, j)
	if err != nil {
		return err
	}
	logs.Info(`ReplicaSet "%s" created`, gjson.Get(resp, "metadata.name"))
	return nil
}

func DelRs(name string) error {
	var err error
	if err = ScaleReplicaSet(name, 0); err != nil {
		return err
	}
	if err = httputil.Delete(fmt.Sprintf(k8sRsURL+"/%s", masterHost, Namespace, name), defaultk8sReqTimeout); err != nil {
		return err
	}
	logs.Warn(`ReplicaSet "%s" deleted`, name)
	return nil
}
