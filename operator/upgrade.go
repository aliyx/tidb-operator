package operator

import (
	"fmt"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
	"k8s.io/client-go/pkg/api/v1"
)

// Upgrade tidb version
func (db *Db) Upgrade() (err error) {
	if db.Status.UpgradeState == upgrading {
		return fmt.Errorf("db %s is upgrading", db.GetName())
	}
	db.Status.UpgradeState = upgrading
	if err = db.update(); err != nil {
		return err
	}
	go func() {
		if !db.TryLock() {
			return
		}
		defer db.Unlock()
		// double-check
		if new, _ := GetDb(db.GetName()); new == nil || new.Status.UpgradeState != upgrading {
			logs.Error("db %s was modified before upgrade", db.GetName())
			return
		}

		defer func() {
			st := upgradeOk
			if err != nil {
				st = upgradeFailed
				logs.Error("failed to upgrade db %s %v", db.GetName(), err)
			}
			db.Status.UpgradeState = st

			if err = db.update(); err != nil {
				logs.Error("failed to update db: %v", err)
			}
		}()
		if err = db.Pd.upgrade(); err != nil {
			return
		}
		if err = db.Tikv.upgrade(); err != nil {
			return
		}
		if err = db.Tidb.upgrade(); err != nil {
			return
		}
	}()
	return nil
}

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

	logs.Info("start upgrading the %v from %s to %s", name, oldversion, version)
	pod.Spec.Containers[0].Image = image
	k8sutil.SetTidbVersion(pod, version)

	patchdata, err := k8sutil.CreatePatch(oldpod, pod, v1.Pod{})
	if err != nil {
		return false, fmt.Errorf("error creating patch: %v", err)
	}

	if err = k8sutil.PatchPod(name, patchdata, waitPodRuningTimeout); err != nil {
		return false, err
	}
	logs.Info("end upgrading the pod %v", name)
	return true, nil
}

func needUpgrade(pod *v1.Pod, version string) bool {
	return k8sutil.GetImageVersion(pod.Spec.Containers[0].Image) != version
}
