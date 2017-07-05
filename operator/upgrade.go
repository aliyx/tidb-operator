package operator

import (
	"fmt"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
	"k8s.io/client-go/pkg/api/v1"
)

func upgradeOne(name, image, version string) (bool, error) {
	pod, err := k8sutil.GetPod(name)
	if err != nil {
		return false, err
	}

	if !needUpgrade(pod, version) {
		return false, nil
	}

	oldversion := k8sutil.GetTidbVersion(pod)
	oldpod := k8sutil.ClonePod(pod)

	logs.Info("upgrading the %v from %s to %s", name, oldversion, version)
	pod.Spec.Containers[0].Image = image
	k8sutil.SetTidbVersion(pod, version)

	patchdata, err := k8sutil.CreatePatch(oldpod, pod, v1.Pod{})
	if err != nil {
		return false, fmt.Errorf("error creating patch: %v", err)
	}

	if err = k8sutil.PatchPod(name, patchdata); err != nil {
		return false, err
	}
	logs.Info("finished upgrading the pod %v", name)
	return true, nil
}

func needUpgrade(pod *v1.Pod, version string) bool {
	return k8sutil.GetImageVersion(pod.Spec.Containers[0].Image) != version
}
