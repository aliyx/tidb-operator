package k8sutil

import (
	"fmt"
	"net/url"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/pkg/httputil"
	"github.com/ghodss/yaml"
	"github.com/tidwall/gjson"
)

func ScaleReplicaSet(name string, replicas int) error {
	k8sMu.Lock()
	defer k8sMu.Unlock()
	if replicas < 0 {
		replicas = 0
	}
	pt := []byte(fmt.Sprintf(rcPatch, replicas))
	err := httputil.Patch(fmt.Sprintf(k8sRsURL+"/%s/scale", masterHost, Namespace, name), pt, defaultk8sReqTimeout)
	if err != nil {
		return err
	}
	logs.Info(`Scale "%s" replicaset: %d`, replicas)
	return nil
}

func CreateRc(s string) error {
	k8sMu.Lock()
	defer k8sMu.Unlock()
	logs.Info("yaml: %s", s)
	url := fmt.Sprintf(k8sRcURL, masterHost, Namespace)
	j, _ := yaml.YAMLToJSON([]byte(s))
	resp, err := httputil.Post(url, j)
	if err != nil {
		return err
	}
	logs.Info(`Replicationcontroller "%s" created`, gjson.Get(resp, "metadata.name"))
	return nil
}

func GetRc(name string) ([]byte, error) {
	resp, err := httputil.Get(fmt.Sprintf(k8sRcURL+"/%s", name, masterHost, Namespace), time.Second)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func DelRc(name string) error {
	var err error
	if err = ScaleReplicationcontroller(name, 0); err != nil {
		return err
	}
	if err = httputil.Delete(fmt.Sprintf(k8sRcURL+"/%s", masterHost, Namespace, name), defaultk8sReqTimeout); err != nil {
		return err
	}
	logs.Warn(`Replicationcontroller "%s" deleted`, name)
	return nil
}

func DelReplicasets(params url.Values) error {
	var queryString string
	if params != nil {
		queryString = params.Encode()
	}
	uri := fmt.Sprintf(k8sRsURL+"?%s", masterHost, Namespace, queryString)
	if err := httputil.Delete(uri, defaultk8sReqTimeout); err != nil {
		return err
	}
	logs.Warn(`Replicasets "%s" deleted`, queryString)

	if err := delPods(params); err != nil {
		return err
	}
	return nil
}
