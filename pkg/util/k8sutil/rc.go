package k8sutil

import (
	"encoding/json"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/util/retryutil"

	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
)

// ScaleReplicationController scale rc
func ScaleReplicationController(name string, replicas int) error {
	var r int32
	if replicas < 0 {
		r = 0
	} else {
		r = int32(replicas)
	}
	return PatchRc(name, func(rc *v1.ReplicationController) {
		rc.Spec.Replicas = &r
	})
}

// CreateRcByJSON create a rc
func CreateRcByJSON(
	j []byte,
	timeout time.Duration,
	updateFunc func(*v1.ReplicationController)) (*v1.ReplicationController, error) {
	rc := &v1.ReplicationController{}
	if err := json.Unmarshal(j, rc); err != nil {
		return nil, err
	}
	updateFunc(rc)
	retRc, err := CreateAndWaitRc(rc, timeout)
	if err != nil {
		return nil, err
	}
	logs.Info("ReplicationController %q created", rc.GetName())
	return retRc, nil
}

// CreateAndWaitRc create a rc and wait all pod is running
func CreateAndWaitRc(rc *v1.ReplicationController, timeout time.Duration) (*v1.ReplicationController, error) {
	retRc, err := kubecli.CoreV1().ReplicationControllers(Namespace).Create(rc)
	if err != nil {
		return nil, err
	}
	interval := 3 * time.Second
	err = retryutil.Retry(interval, int(timeout/(interval)), func() (bool, error) {
		retRc, err = kubecli.CoreV1().ReplicationControllers(Namespace).Get(rc.GetName(), metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if retRc.Status.AvailableReplicas != *rc.Spec.Replicas {
			return false, nil
		}
		return true, nil
	})
	logs.Info("ReplicationController %q created", rc.GetName())
	return retRc, nil
}

func cloneRc(d *v1.ReplicationController) *v1.ReplicationController {
	cr, err := scheme.Scheme.DeepCopy(d)
	if err != nil {
		panic("cannot deep copy pod")
	}
	return cr.(*v1.ReplicationController)
}

// PatchRc patch a ReplicationController
func PatchRc(name string, updateFunc func(*v1.ReplicationController)) error {
	or, err := kubecli.CoreV1().ReplicationControllers(Namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	nr := cloneRc(or)
	updateFunc(nr)
	patchData, err := CreatePatch(or, nr, v1.ReplicationController{})
	if err != nil {
		return err
	}
	_, err = kubecli.CoreV1().ReplicationControllers(Namespace).Patch(name, types.StrategicMergePatchType, patchData)
	return err
}

// DelRc cascade delete rc and it's pods
func DelRc(name string) (err error) {
	err = kubecli.CoreV1().ReplicationControllers(Namespace).Delete(name, CascadeDeleteOptions(5))
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	logs.Info("ReplicationController %q deleted", name)
	return nil
}
