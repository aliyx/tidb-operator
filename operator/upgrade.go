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

	logs.Info("start upgrading %v from %s to %s", name, k8sutil.GetTidbVersion(pod), version)

	if err = k8sutil.PatchPod(pod, func(np *v1.Pod) {
		np.Spec.Containers[0].Image = image
		k8sutil.SetTidbVersion(np, version)
	}, waitPodRuningTimeout); err != nil {
		return false, err
	}
	logs.Info("end upgrading %v", name)
	return true, nil
}

func upgradeRC(name, image, version string) error {
	err := k8sutil.PatchRc(name, func(rc *v1.ReplicationController) {
		logs.Info("start upgrading replicationcontroller(%s) from %s to %s", name, k8sutil.GetTidbVersion(rc), version)
		k8sutil.SetTidbVersion(rc, version)
		rc.Spec.Template.Spec.Containers[0].Image = image
	})
	logs.Info("end upgrading %s", name)
	return err
}

func needUpgrade(i interface{}, version string) bool {
	image := ""
	switch v := i.(type) {
	case *v1.Pod:
		image = k8sutil.GetImageVersion(v.Spec.Containers[0].Image)
	case *v1.ReplicationController:
		image = v.Spec.Template.Spec.Containers[0].Image
	}
	return k8sutil.GetImageVersion(image) != version
}
