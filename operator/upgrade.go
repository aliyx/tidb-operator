package operator

import (
	"fmt"

	"strings"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
	"k8s.io/client-go/pkg/api/v1"
)

func upgradeOne(name, image string) error {
	pod, err := k8sutil.GetPod(name)
	if err != nil {
		return err
	}
	oldpod := k8sutil.ClonePod(pod)

	sp := strings.Split(image, ":")
	logs.Info("upgrading the %v from %s to %s", name, k8sutil.GetTidbVersion(pod), sp[len(sp)-1])
	pod.Spec.Containers[0].Image = image

	patchdata, err := k8sutil.CreatePatch(oldpod, pod, v1.Pod{})
	if err != nil {
		return fmt.Errorf("error creating patch: %v", err)
	}

	if err = k8sutil.PatchPod(name, patchdata); err != nil {
		return err
	}
	logs.Info("finished upgrading the %v", name)
	return nil
}
