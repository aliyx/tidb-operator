package k8sutil

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
	"github.com/ffan/tidb-k8s/pkg/httputil"
	"github.com/ffan/tidb-k8s/pkg/retryutil"
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

// Net ip and port
type Net struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

// String to ip:port
func (n Net) String() string {
	return fmt.Sprintf("%s:%d", n.IP, n.Port)
}

// K8sInfo describe k8s component
type K8sInfo struct {
	CPU      int    `json:"cpu"`
	Mem      int    `json:"mem"`
	Version  string `json:"version"`
	Replicas int    `json:"replicas"`

	Nets []Net `json:"nets,omitempty"`
	Ok   bool  `json:"ok,omitempty"`
}

func (k *K8sInfo) Validate() error {
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

func CreatePod(s string) error {
	k8sMu.Lock()
	defer k8sMu.Unlock()
	logs.Info("yaml: %s", s)
	url := fmt.Sprintf(k8sPodsURL, masterHost, Namespace)
	j, _ := yaml.YAMLToJSON([]byte(s))
	resp, err := httputil.Post(url, j)
	if err != nil {
		return err
	}
	logs.Info(`Pod "%s" created`, gjson.Get(resp, "metadata.name"))
	return nil
}

func GetPod(name string) ([]byte, error) {
	resp, err := httputil.Get(fmt.Sprintf(k8sPodsURL+"/%s", masterHost, Namespace, name), time.Second)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// getPodProperties 返回指定template的值
func GetPodProperties(name, tem string) (string, error) {
	resp, err := GetPod(name)
	if err != nil {
		return "", err
	}
	return execTemplate(name, tem, resp)
}

func isPodOk(name string) (string, bool) {
	resp, err := GetPod(name)
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

func WaitPodsRuning(timeout time.Duration, names ...string) (err error) {
	retryutil.RetryIfErr(timeout, func() error {
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

func WaitComponentRuning(timeout time.Duration, cell, component string) (err error) {
	pods, err := ListPodNames(cell, component)
	logs.Info(`%s "%s" pods: %v`, component, cell, pods)
	if err != nil {
		return err
	}
	if err := WaitPodsRuning(timeout, pods...); err != nil {
		return err
	}
	logs.Info(`%s "%s" has ok`, component, cell)
	return nil
}

func DelPodsBy(cell, component string) error {
	params := url.Values{}
	setLabelSelector(params, cell, component)
	qs := params.Encode()
	uri := fmt.Sprintf(k8sPodsURL+"?%s", masterHost, Namespace, qs)
	if err := httputil.Delete(uri, defaultk8sReqTimeout); err != nil {
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
	uri := fmt.Sprintf(k8sPodsURL+"?%s", masterHost, Namespace, queryString)
	if err := httputil.Delete(uri, defaultk8sReqTimeout); err != nil {
		return err
	}
	logs.Warn(`Pods "%s" deleted`, queryString)
	return nil
}

func ListPodNames(cell, component string) ([]string, error) {
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
	uri := fmt.Sprintf(k8sPodsURL+"?%s", masterHost, Namespace, queryString)
	bs, err := httputil.Get(uri, defaultk8sReqTimeout)
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
