package k8sutil

import (
	"encoding/json"
	"time"

	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/util/retryutil"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
func CreateRcByJSON(j []byte, timeout time.Duration) (*v1.ReplicationController, error) {
	rc := &v1.ReplicationController{}
	if err := json.Unmarshal(j, rc); err != nil {
		return nil, err
	}

	version := GetImageVersion(rc.Spec.Template.Spec.Containers[0].Image)
	SetTidbVersion(rc, version)
	if rc.Spec.Template.Annotations == nil {
		rc.Spec.Template.Annotations = make(map[string]string)
	}
	rc.Spec.Template.Annotations[tidbVersionAnnotationKey] = version
	retRc, err := kubecli.CoreV1().ReplicationControllers(Namespace).Create(rc)
	if err != nil {
		return nil, err
	}
	logs.Info("ReplicationController '%s' created", rc.GetName())
	return retRc, nil
}

// CreateAndwaitRc create a rc and wait all pod is running
func CreateAndwaitRc(rc *v1.ReplicationController, timeout time.Duration) (*v1.ReplicationController, error) {
	_, err := kubecli.CoreV1().ReplicationControllers(Namespace).Create(rc)
	if err != nil {
		return nil, err
	}
	interval := 3 * time.Second
	var retRc *v1.ReplicationController
	err = retryutil.Retry(interval, int(timeout/(interval)), func() (bool, error) {
		retRc, err = kubecli.CoreV1().ReplicationControllers(Namespace).Get(rc.GetName(), meta_v1.GetOptions{})
		if err != nil {
			return false, err
		}
		if retRc.Status.AvailableReplicas != *rc.Spec.Replicas {
			return false, nil
		}
		return true, nil
	})
	logs.Info(`ReplicationController "%s" created`, rc.GetName())
	return retRc, nil
}

func cloneRc(d *v1.ReplicationController) *v1.ReplicationController {
	cr, err := api.Scheme.DeepCopy(d)
	if err != nil {
		panic("cannot deep copy pod")
	}
	return cr.(*v1.ReplicationController)
}

// PatchRc patch a ReplicationController
func PatchRc(name string, updateFunc func(*v1.ReplicationController)) error {
	or, err := kubecli.CoreV1().ReplicationControllers(Namespace).Get(name, meta_v1.GetOptions{})
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
	kubecli.CoreV1().ReplicationControllers(Namespace).Delete(name, CascadeDeleteOptions(5))
	logs.Warn(`ReplicationController "%s" deleted`, name)
	return nil
}
