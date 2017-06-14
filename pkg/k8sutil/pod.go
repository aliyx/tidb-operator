package k8sutil

import (
	"encoding/json"
	"fmt"

	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/pkg/retryutil"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
)

// CreateAndWaitPodByJSON create and wait pod status 'running'
func CreateAndWaitPodByJSON(j []byte, timeout time.Duration) (*v1.Pod, error) {
	pod := &v1.Pod{}
	if err := json.Unmarshal(j, pod); err != nil {
		return nil, err
	}
	return CreateAndWaitPod(pod, timeout)
}

// CreateAndWaitPod create and wait pod status 'running'
func CreateAndWaitPod(pod *v1.Pod, timeout time.Duration) (*v1.Pod, error) {
	_, err := kubecli.CoreV1().Pods(Namespace).Create(pod)
	if err != nil {
		return nil, err
	}

	interval := time.Second
	var retPod *v1.Pod
	err = retryutil.Retry(interval, int(timeout/(interval)), func() (bool, error) {
		retPod, err = kubecli.CoreV1().Pods(Namespace).Get(pod.Name, meta_v1.GetOptions{})
		if err != nil {
			return false, err
		}
		switch retPod.Status.Phase {
		case v1.PodRunning:
			return true, nil
		case v1.PodPending:
			return false, nil
		default:
			return false, fmt.Errorf("unexpected pod status.phase: %v", retPod.Status.Phase)
		}
	})
	logs.Info(`Pod "%s" created`, retPod.GetName())
	return retPod, err
}

// ClonePod deep clone Pod
func ClonePod(p *v1.Pod) *v1.Pod {
	np, err := api.Scheme.DeepCopy(p)
	if err != nil {
		panic("cannot deep copy pod")
	}
	return np.(*v1.Pod)
}

// DeletePods delete the specified names pod
func DeletePods(podNames ...string) error {
	for _, pName := range podNames {
		if err := kubecli.CoreV1().Pods(Namespace).Delete(pName, meta_v1.NewDeleteOptions(0)); err != nil {
			return err
		}
		logs.Info(`Pod "%s" deleted`, pName)
	}
	return nil
}

// DeletePodsBy delete the specified cell pods
func DeletePodsBy(cell, component string) error {
	option := meta_v1.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"cell":      cell,
			"component": component,
		}).String(),
	}
	if err := kubecli.CoreV1().Pods(Namespace).DeleteCollection(meta_v1.NewDeleteOptions(0), option); err != nil {
		return err
	}
	logs.Warn(`Pods cell:"%s" component:"%s" deleted`, cell, component)
	return nil
}

// GetPods Gets the pods of the specified cell and component
func GetPods(cell, component string) ([]v1.Pod, error) {
	set := make(map[string]string)
	if cell != "" {
		set["cell"] = cell
	}
	if component != "" {
		set["component"] = component
	}
	opts := meta_v1.ListOptions{
		LabelSelector: labels.SelectorFromSet(set).String(),
	}
	list, err := kubecli.CoreV1().Pods(Namespace).List(opts)
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetPodNames get specified pods name
func GetPodNames(pods []v1.Pod) []string {
	if len(pods) == 0 {
		return nil
	}
	res := []string{}
	for _, p := range pods {
		res = append(res, p.Name)
	}
	return res
}

// ListPodNames get specified pods name
func ListPodNames(cell, component string) ([]string, error) {
	pods, err := GetPods(cell, component)
	if err != nil {
		return nil, err
	}
	return GetPodNames(pods), nil
}

// PodWithNodeSelector set pod nodeselecter
func PodWithNodeSelector(p *v1.Pod, ns map[string]string) *v1.Pod {
	p.Spec.NodeSelector = ns
	return p
}
