package k8sutil

import (
	"fmt"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/pkg/httputil"
	"github.com/ghodss/yaml"
	"github.com/tidwall/gjson"
)

func GetService(name string) ([]byte, error) {
	resp, err := httputil.Get(fmt.Sprintf(k8sServiceURL+"/%s", masterHost, Namespace, name), time.Second)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// getServiceProperties 获取template中的数据
func GetServiceProperties(name, tem string) (string, error) {
	resp, err := GetService(name)
	if err != nil {
		return "", err
	}
	return execTemplate(name, tem, resp)
}

func CreateService(s string) error {
	k8sMu.Lock()
	defer k8sMu.Unlock()
	logs.Info("yaml: %s", s)
	url := fmt.Sprintf(k8sServiceURL, masterHost, Namespace)
	j, _ := yaml.YAMLToJSON([]byte(s))
	resp, err := httputil.Post(url, j)
	if err != nil {
		return err
	}
	logs.Info(`Service "%s" created`, gjson.Get(resp, "metadata.name"))
	return nil
}

func DelSrvs(names ...string) error {
	for _, name := range names {
		uri := fmt.Sprintf(k8sServiceURL+"/%s", masterHost, Namespace, name)
		if err := httputil.Delete(uri, defaultk8sReqTimeout); err != nil {
			return err
		}
		logs.Warn(`Service "%s" deleted`, name)
	}
	return nil
}
