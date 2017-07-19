package k8sutil

import (
	"encoding/json"
	"fmt"
	"strings"

	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/util/retryutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
)

var (
	tidbVersionAnnotationKey = "tidb.version"
)

// GetTidbVersion get tidb image version
func GetTidbVersion(pod *v1.Pod) string {
	return pod.Annotations[tidbVersionAnnotationKey]
}

// SetTidbVersion set tidb image version
func SetTidbVersion(pod *v1.Pod, version string) {
	if len(pod.Annotations) < 1 {
		pod.Annotations = make(map[string]string)
	}
	pod.Annotations[tidbVersionAnnotationKey] = version
}

// CreateAndWaitPodByJSON create and wait pod status 'running'
func CreateAndWaitPodByJSON(j []byte, timeout time.Duration) (*v1.Pod, error) {
	pod := &v1.Pod{}
	if err := json.Unmarshal(j, pod); err != nil {
		return nil, err
	}
	SetTidbVersion(pod, GetImageVersion(pod.Spec.Containers[0].Image))
	return CreateAndWaitPod(pod, timeout)
}

// GetImageVersion get version in image
func GetImageVersion(image string) string {
	sp := strings.Split(image, ":")
	return sp[len(sp)-1]
}

// PatchPod path pod
func PatchPod(name string, patchdata []byte, timeout time.Duration) error {
	_, err := kubecli.CoreV1().Pods(Namespace).Patch(name, types.StrategicMergePatchType, patchdata)
	if err != nil {
		return fmt.Errorf("fail to update the pod (%s): %v", name, err)
	}
	// wait restart the pod
	time.Sleep(3 * time.Second)
	_, err = waitPodRunning(name, timeout)
	if err != nil {
		return err
	}

	return nil
}

func waitPodRunning(name string, timeout time.Duration) (*v1.Pod, error) {
	var (
		err      error
		retPod   *v1.Pod
		interval = 2 * time.Second
	)
	return retPod, retryutil.Retry(interval, int(timeout/(interval)), func() (bool, error) {
		retPod, err = kubecli.CoreV1().Pods(Namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		switch retPod.Status.Phase {
		case v1.PodRunning:
			return true, nil
		case v1.PodPending:
			for _, c := range retPod.Status.Conditions {
				if c.Reason == v1.PodReasonUnschedulable {
					return false, fmt.Errorf("%s:%s", c.Reason, c.Message)
				}
			}
			return false, nil
		default:
			return false, fmt.Errorf("unexpected pod status.phase: %v", retPod.Status.Phase)
		}
	})
}

// CreateAndWaitPod create and wait pod status 'running'
func CreateAndWaitPod(pod *v1.Pod, timeout time.Duration) (*v1.Pod, error) {
	retPod, err := kubecli.CoreV1().Pods(Namespace).Create(pod)
	if err != nil {
		return nil, err
	}

	retPod, err = waitPodRunning(pod.GetName(), timeout)
	if err != nil {
		return nil, err
	}
	logs.Info("Pod '%s' created", retPod.GetName())
	return retPod, err
}

func IsPodRunning(pod v1.Pod) bool {
	return pod.Status.Phase == v1.PodRunning
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
		if err := kubecli.CoreV1().Pods(Namespace).Delete(pName, metav1.NewDeleteOptions(0)); err != nil {
			return err
		}
		logs.Info(`Pod "%s" deleted`, pName)
	}
	return nil
}

// DeletePodsBy delete the specified cell pods
func DeletePodsBy(cell, component string) error {
	option := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"cell":      cell,
			"component": component,
		}).String(),
	}
	if err := kubecli.CoreV1().Pods(Namespace).DeleteCollection(metav1.NewDeleteOptions(0), option); err != nil {
		return err
	}
	logs.Warn(`Pods cell:"%s" component:"%s" deleted`, cell, component)
	return nil
}

// DeletePodsByLabel delete the specified label pods
func DeletePodsByLabel(ls map[string]string) error {
	option := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(ls).String(),
	}
	if err := kubecli.CoreV1().Pods(Namespace).DeleteCollection(metav1.NewDeleteOptions(0), option); err != nil {
		return err
	}
	logs.Warn("Pods '%s' deleted", ls)
	return nil
}

// GetPodsByNamespace Gets the pods of the specified namespace
func GetPodsByNamespace(ns string, ls map[string]string) ([]v1.Pod, error) {
	opts := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(ls).String(),
	}
	list, err := kubecli.CoreV1().Pods(ns).List(opts)
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetPod Get the pods of the specified name
func GetPod(name string) (*v1.Pod, error) {
	return kubecli.CoreV1().Pods(Namespace).Get(name, metav1.GetOptions{})
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
	opts := metav1.ListOptions{
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
