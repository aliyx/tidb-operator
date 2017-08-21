package k8sutil

import (
	"encoding/json"
	"fmt"
	"strings"

	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/util/retryutil"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
)

// CreatePodByJSON create and wait pod status 'running'
func CreatePodByJSON(j []byte, timeout time.Duration, updateFunc func(*v1.Pod)) (*v1.Pod, error) {
	pod := &v1.Pod{}
	if err := json.Unmarshal(j, pod); err != nil {
		return nil, err
	}
	updateFunc(pod)
	return CreateAndWaitPod(pod, timeout)
}

// CreateAndWaitPodByJSON create and wait pod status 'running'
func CreateAndWaitPodByJSON(j []byte, timeout time.Duration) (*v1.Pod, error) {
	pod := &v1.Pod{}
	if err := json.Unmarshal(j, pod); err != nil {
		return nil, err
	}
	return CreateAndWaitPod(pod, timeout)
}

// PatchPod path pod
func PatchPod(op *v1.Pod, timeout time.Duration, updateFunc func(*v1.Pod)) error {
	np := clonePod(op)
	updateFunc(np)
	patchData, err := CreatePatch(op, np, v1.Pod{})
	if err != nil {
		return err
	}
	_, err = kubecli.CoreV1().Pods(Namespace).Patch(op.GetName(), types.StrategicMergePatchType, patchData)
	if err != nil {
		return err
	}
	// check pod status after old pod killed, time is 'TerminationGracePeriodSeconds'
	time.Sleep(time.Duration(*op.Spec.TerminationGracePeriodSeconds+3) * time.Second)
	_, err = waitPodRunning(op.GetName(), timeout)
	if err != nil {
		return err
	}
	return nil
}

func waitPodRunning(name string, timeout time.Duration) (*v1.Pod, error) {
	var (
		err      error
		retPod   *v1.Pod
		interval = 3 * time.Second
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
				// No free memery or cpu etc.
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
	logs.Info("Pod %q created", retPod.GetName())
	retPod, err = waitPodRunning(pod.GetName(), timeout)
	if err != nil {
		return nil, err
	}
	return retPod, err
}

// IsPodOk pod status is True
func IsPodOk(pod v1.Pod) (ok bool) {
	ok = true
	if pod.Status.Phase != v1.PodRunning {
		ok = false
	}
	if !ok {
		return
	}
	for _, c := range pod.Status.Conditions {
		if c.Status != v1.ConditionTrue {
			ok = false
			return
		}
	}
	return
}

func clonePod(p *v1.Pod) *v1.Pod {
	np, err := api.Scheme.DeepCopy(p)
	if err != nil {
		panic("cannot deep copy pod")
	}
	return np.(*v1.Pod)
}

// DeletePods delete the specified names pod
func DeletePods(podNames ...string) error {
	for _, pName := range podNames {
		err := kubecli.CoreV1().Pods(Namespace).Delete(pName, &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		logs.Info(`Pod "%s" deleted`, pName)
	}
	return nil
}

// DeletePod delete the specified pod
func DeletePod(name string, timeout int64) error {
	err := kubecli.CoreV1().Pods(Namespace).Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logs.Warn("to delete a nonexistent pod %q", name)
			return nil
		}
		return err
	}
	time.Sleep(time.Duration(timeout) * time.Second)
	// Do not know why can also get the pod after that is delete and over timeout
	if p, _ := GetPod(name); p == nil || p.GetName() != name {
		logs.Info("Pod %q deleted", name)
		return nil
	}
	return fmt.Errorf("Pod %q not been deleted", name)
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
	logs.Info("Pods cell: %q component: %q deleted", cell, component)
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
	logs.Info(strings.Replace(fmt.Sprintf("Pods %q deleted", ls), "map[", "label[", -1))
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
	set["app"] = "tidb"
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
