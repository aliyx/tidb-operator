package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"text/template"

	"time"

	"strings"

	"net/url"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/models/utils"
	"github.com/ghodss/yaml"
	"github.com/tidwall/gjson"
)

var (
	k8sServiceURL    = "%s/api/v1/namespaces/%s/services"
	k8sRcURL         = "%s/api/v1/namespaces/%s/replicationcontrollers"
	k8sRsURL         = "%s/apis/extensions/v1beta1/namespaces/%s/replicasets"
	k8sDeploymentURL = "%s/apis/extensions/v1beta1/namespaces/%s/deployments"
	k8sPodsURL       = "%s/api/v1/namespaces/%s/pods"
	k8sNamespaceURL  = "%s/api/v1/namespaces"

	rcPatch = `{"spec": {"replicas": %d}}`

	errPodScheduled = "Unschedulable"

	k8sMu sync.Mutex
)

// Net tidb子模块对外ip和port
type Net struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

// String to ip:port
func (n Net) String() string {
	return fmt.Sprintf("%s:%d", n.IP, n.Port)
}

// K8sInfo 描述pd/tikv/tidb在kubernertes上的信息
type K8sInfo struct {
	CPU      int    `json:"cpu"`
	Mem      int    `json:"mem"`
	Version  string `json:"version"`
	Replicas int    `json:"replicas"`

	Nets []Net `json:"nets,omitempty"`
	Ok   bool  `json:"ok,omitempty"`
}

func (k *K8sInfo) validate() error {
	if k.CPU < 200 || k.CPU > 2000 {
		return fmt.Errorf("cpu must be between 200-2000")
	}
	if k.Mem < 256 || k.CPU > 8184 {
		return fmt.Errorf("cpu must be between 256-8184")
	}
	if k.Replicas < 1 {
		return fmt.Errorf("replicas must be greater than 1")
	}
	return nil
}

func createNamespace(s string) error {
	k8sMu.Lock()
	defer k8sMu.Unlock()
	logs.Info("yaml: %s", s)
	url := fmt.Sprintf(k8sNamespaceURL, k8sAddr)
	j, _ := yaml.YAMLToJSON([]byte(s))
	resp, err := utils.Post(url, j)
	if err != nil {
		return err
	}
	logs.Info(`Namespace "%s" created`, gjson.Get(resp, "metadata.name"))
	return nil
}

func delNamespace(name string) error {
	var err error
	if err = utils.Delete(fmt.Sprintf(k8sNamespaceURL+"/%s", k8sAddr, name), k8sAPITimeout); err != nil {
		return err
	}
	logs.Warn(`Namespace "%s" deleted`, name)
	return nil
}

func getService(name string) ([]byte, error) {
	resp, err := utils.Get(fmt.Sprintf(k8sServiceURL+"/%s", k8sAddr, getNamespace(), name), time.Second)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// getServiceProperties 获取template中的数据
func getServiceProperties(name, tem string) (string, error) {
	resp, err := getService(name)
	if err != nil {
		return "", err
	}
	return execTemplate(name, tem, resp)
}

func createService(s string) error {
	k8sMu.Lock()
	defer k8sMu.Unlock()
	logs.Info("yaml: %s", s)
	url := fmt.Sprintf(k8sServiceURL, k8sAddr, getNamespace())
	j, _ := yaml.YAMLToJSON([]byte(s))
	resp, err := utils.Post(url, j)
	if err != nil {
		return err
	}
	logs.Info(`Service "%s" created`, gjson.Get(resp, "metadata.name"))
	return nil
}

func delSrvs(names ...string) error {
	for _, name := range names {
		uri := fmt.Sprintf(k8sServiceURL+"/%s", k8sAddr, getNamespace(), name)
		if err := utils.Delete(uri, k8sAPITimeout); err != nil {
			return err
		}
		logs.Warn(`Service "%s" deleted`, name)
	}
	return nil
}

func scaleReplicationcontroller(name string, replicas int) error {
	k8sMu.Lock()
	defer k8sMu.Unlock()
	if replicas < 0 {
		replicas = 0
	}
	pt := []byte(fmt.Sprintf(rcPatch, replicas))
	err := utils.Patch(fmt.Sprintf(k8sRcURL+"/%s/scale", k8sAddr, getNamespace(), name), pt, k8sAPITimeout)
	if err != nil {
		return err
	}
	logs.Info(`Scale "%s" replicationcontroller: %d`, name, replicas)
	return nil
}

func scaleReplicaSet(name string, replicas int) error {
	k8sMu.Lock()
	defer k8sMu.Unlock()
	if replicas < 0 {
		replicas = 0
	}
	pt := []byte(fmt.Sprintf(rcPatch, replicas))
	err := utils.Patch(fmt.Sprintf(k8sRsURL+"/%s/scale", k8sAddr, getNamespace(), name), pt, k8sAPITimeout)
	if err != nil {
		return err
	}
	logs.Info(`Scale "%s" replicaset: %d`, replicas)
	return nil
}

func createRc(s string) error {
	k8sMu.Lock()
	defer k8sMu.Unlock()
	logs.Info("yaml: %s", s)
	url := fmt.Sprintf(k8sRcURL, k8sAddr, getNamespace())
	j, _ := yaml.YAMLToJSON([]byte(s))
	resp, err := utils.Post(url, j)
	if err != nil {
		return err
	}
	logs.Info(`Replicationcontroller "%s" created`, gjson.Get(resp, "metadata.name"))
	return nil
}

func getRc(name string) ([]byte, error) {
	resp, err := utils.Get(fmt.Sprintf(k8sRcURL+"/%s", name, k8sAddr, getNamespace()), time.Second)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func delRc(name string) error {
	var err error
	if err = scaleReplicationcontroller(name, 0); err != nil {
		return err
	}
	if err = utils.Delete(fmt.Sprintf(k8sRcURL+"/%s", k8sAddr, getNamespace(), name), k8sAPITimeout); err != nil {
		return err
	}
	logs.Warn(`Replicationcontroller "%s" deleted`, name)
	return nil
}

func createDeployment(s string) error {
	k8sMu.Lock()
	defer k8sMu.Unlock()
	logs.Info("yaml: %s", s)
	url := fmt.Sprintf(k8sDeploymentURL, k8sAddr, getNamespace())
	j, _ := yaml.YAMLToJSON([]byte(s))
	resp, err := utils.Post(url, j)
	if err != nil {
		return err
	}
	logs.Info(`Deployment "%s" created`, gjson.Get(resp, "metadata.name"))
	return nil
}

func delDeployment(name string, cascade bool) error {
	uri := fmt.Sprintf(k8sDeploymentURL+"/%s", k8sAddr, getNamespace(), name)
	if err := utils.Delete(uri, k8sAPITimeout); err != nil {
		return err
	}
	logs.Warn(`Deployment "%s" deleted`, name)
	if cascade {
		params := url.Values{}
		cell := strings.Split(name, "-")[1]
		params.Add("labelSelector", fmt.Sprintf("app=tidb,cell=%s,component=tikv", cell))
		if err := delReplicasets(params); err != nil {
			return err
		}
	}
	return nil
}

func createRs(s string) error {
	k8sMu.Lock()
	defer k8sMu.Unlock()
	logs.Info("yaml: %s", s)
	url := fmt.Sprintf(k8sRsURL, k8sAddr, getNamespace())
	j, _ := yaml.YAMLToJSON([]byte(s))
	resp, err := utils.Post(url, j)
	if err != nil {
		return err
	}
	logs.Info(`ReplicaSet "%s" created`, gjson.Get(resp, "metadata.name"))
	return nil
}

func delRs(name string) error {
	var err error
	if err = scaleReplicaSet(name, 0); err != nil {
		return err
	}
	if err = utils.Delete(fmt.Sprintf(k8sRsURL+"/%s", k8sAddr, getNamespace(), name), k8sAPITimeout); err != nil {
		return err
	}
	logs.Warn(`ReplicaSet "%s" deleted`, name)
	return nil
}

func delReplicasets(params url.Values) error {
	var queryString string
	if params != nil {
		queryString = params.Encode()
	}
	uri := fmt.Sprintf(k8sRsURL+"?%s", k8sAddr, getNamespace(), queryString)
	if err := utils.Delete(uri, k8sAPITimeout); err != nil {
		return err
	}
	logs.Warn(`Replicasets "%s" deleted`, queryString)

	if err := delPods(params); err != nil {
		return err
	}
	return nil
}

func createPod(s string) error {
	k8sMu.Lock()
	defer k8sMu.Unlock()
	logs.Info("yaml: %s", s)
	url := fmt.Sprintf(k8sPodsURL, k8sAddr, getNamespace())
	j, _ := yaml.YAMLToJSON([]byte(s))
	resp, err := utils.Post(url, j)
	if err != nil {
		return err
	}
	logs.Info(`Pod "%s" created`, gjson.Get(resp, "metadata.name"))
	return nil
}

func getPod(name string) ([]byte, error) {
	resp, err := utils.Get(fmt.Sprintf(k8sPodsURL+"/%s", k8sAddr, getNamespace(), name), time.Second)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// getPodProperties 返回指定template的值
func getPodProperties(name, tem string) (string, error) {
	resp, err := getPod(name)
	if err != nil {
		return "", err
	}
	return execTemplate(name, tem, resp)
}

func isPodOk(name string) (string, bool) {
	resp, err := getPod(name)
	if err != nil {
		return fmt.Sprintf("%v", err), false
	}
	str := string(resp)
	ret := gjson.Get(str, "status.phase")
	if ret.String() != "Running" {
		reasons := gjson.Get(str, "status.conditions.#[status==False]#.reason")
		var s string
		if reasons.Exists() {
			s = reasons.String()
		}
		return s, false
	}
	return "", true
}

func waitPodsRuning(timeout time.Duration, names ...string) (err error) {
	utils.RetryIfErr(timeout, func() error {
		for _, name := range names {
			rea, ok := isPodOk(name)
			if ok {
				continue
			}
			// 出错
			if rea != "" {
				if strings.Contains(rea, errPodScheduled) {
					err = fmt.Errorf("insufficient cpu or memory")
					return nil
				}
			}
			// 正在启动 continue
			return fmt.Errorf(`pod "%s" is pending`, name)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func waitComponentRuning(timeout time.Duration, cell, component string) (err error) {
	pods, err := listPodNames(cell, component)
	logs.Info(`%s "%s" pods: %v`, component, cell, pods)
	if err != nil {
		return err
	}
	if err := waitPodsRuning(timeout, pods...); err != nil {
		return err
	}
	logs.Info(`%s "%s" has ok`, component, cell)
	return nil
}

func delPodsBy(cell, component string) error {
	params := url.Values{}
	setLabelSelector(params, cell, component)
	qs := params.Encode()
	uri := fmt.Sprintf(k8sPodsURL+"?%s", k8sAddr, getNamespace(), qs)
	if err := utils.Delete(uri, k8sAPITimeout); err != nil {
		return err
	}
	logs.Warn(`Pods "%s" deleted`, qs)
	return nil
}

func delPods(params url.Values) error {
	var queryString string
	if params != nil {
		queryString = params.Encode()
	}
	uri := fmt.Sprintf(k8sPodsURL+"?%s", k8sAddr, getNamespace(), queryString)
	if err := utils.Delete(uri, k8sAPITimeout); err != nil {
		return err
	}
	logs.Warn(`Pods "%s" deleted`, queryString)
	return nil
}

func listPodNames(cell, component string) ([]string, error) {
	params := url.Values{}
	labels := "app=tidb"
	if cell != "" {
		labels = fmt.Sprintf("%s,cell=%s", labels, cell)
	}
	if component != "" {
		labels = fmt.Sprintf("%s,component=%s", labels, component)
	}
	params.Add("labelSelector", labels)
	queryString := params.Encode()
	uri := fmt.Sprintf(k8sPodsURL+"?%s", k8sAddr, getNamespace(), queryString)
	bs, err := utils.Get(uri, k8sAPITimeout)
	if err != nil {
		return nil, err
	}
	result := gjson.Get(string(bs), "items.#.metadata.name")
	if result.Type == gjson.Null {
		return nil, fmt.Errorf("cannt get pods")
	}
	var pods []string
	for _, name := range result.Array() {
		pods = append(pods, name.String())
	}
	return pods, nil
}

func execTemplate(name, tem string, data []byte) (string, error) {
	var objmap map[string]interface{}
	if err := json.Unmarshal(data, &objmap); err != nil {
		return "", err
	}
	tmpl, err := template.New(name).Parse(tem)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	w := io.MultiWriter(&buf)
	if err := tmpl.Execute(w, objmap); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func setLabelSelector(params url.Values, cell, component string) {
	params.Add("labelSelector", fmt.Sprintf("app=tidb,cell=%s,component=%s", cell, component))
}
