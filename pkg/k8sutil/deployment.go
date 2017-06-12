package k8sutil

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/pkg/httputil"
	"github.com/ghodss/yaml"
	"github.com/tidwall/gjson"
)

func CreateDeployment(s string) error {
	k8sMu.Lock()
	defer k8sMu.Unlock()
	logs.Info("yaml: %s", s)
	url := fmt.Sprintf(k8sDeploymentURL, masterHost, Namespace)
	j, _ := yaml.YAMLToJSON([]byte(s))
	resp, err := httputil.Post(url, j)
	if err != nil {
		return err
	}
	logs.Info(`Deployment "%s" created`, gjson.Get(resp, "metadata.name"))
	return nil
}

func DelDeployment(name string, cascade bool) error {
	uri := fmt.Sprintf(k8sDeploymentURL+"/%s", masterHost, Namespace, name)
	if err := httputil.Delete(uri, defaultk8sReqTimeout); err != nil {
		return err
	}
	logs.Warn(`Deployment "%s" deleted`, name)
	if cascade {
		params := url.Values{}
		cell := strings.Split(name, "-")[1]
		params.Add("labelSelector", fmt.Sprintf("app=tidb,cell=%s,component=tikv", cell))
		if err := DelReplicasets(params); err != nil {
			return err
		}
	}
	return nil
}
