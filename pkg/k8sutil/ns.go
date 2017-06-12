package k8sutil

import (
	"fmt"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/pkg/httputil"
	"github.com/ghodss/yaml"
	"github.com/tidwall/gjson"
)

func createNamespace(s string) error {
	k8sMu.Lock()
	defer k8sMu.Unlock()
	logs.Info("yaml: %s", s)
	url := fmt.Sprintf(k8sNamespaceURL, masterHost)
	j, _ := yaml.YAMLToJSON([]byte(s))
	resp, err := httputil.Post(url, j)
	if err != nil {
		return err
	}
	logs.Info(`Namespace "%s" created`, gjson.Get(resp, "metadata.name"))
	return nil
}

func DelNamespace(name string) error {
	var err error
	if err = httputil.Delete(fmt.Sprintf(k8sNamespaceURL+"/%s", masterHost, name), defaultk8sReqTimeout); err != nil {
		return err
	}
	logs.Warn(`Namespace "%s" deleted`, name)
	return nil
}
