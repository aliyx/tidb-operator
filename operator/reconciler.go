package operator

import (
	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/storage"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
	"k8s.io/client-go/pkg/api/v1"
)

// Reconcile tikv and tidb desired status
// 1. reconcile replica count
// 2. reconcile version
func (db *Db) Reconcile() (err error) {
	if !db.TryLock() {
		return
	}
	defer db.Unlock()

	defer func() {
		if err != nil {
			db.Event(eventDb, "reconcile").Trace(err, "Failed to reconcile tidb cluster")
		}
	}()

	// check is scaling
	if db.Status.ScaleState&scaling > 0 {
		return
	}

	if !db.Status.Available {
		err = ErrUnavailable
		return
	}

	logs.Debug("start reconciling db", db.GetName())
	db.Status.ScaleState |= scaling
	if err = db.update(); err != nil {
		return
	}
	defer func() {
		parseError(db, err)
		db.Status.ScaleState ^= scaling
		if err = db.update(); err != nil {
			logs.Error("failed to update db %s: %v", db.GetName(), err)
		} else {
			logs.Debug("end reconciling db", db.GetName())
		}
		if err == nil {
			db.upgrade()
		}
	}()

	if err = db.reconcilePds(); err != nil {
		return
	}
	if err = db.reconcileTikvs(); err != nil {
		return
	}
	if err = db.reconcileTidbs(); err != nil {
		return
	}
	return
}

// upgrade tidb version
func (db *Db) upgrade() (err error) {
	defer func() {
		if err != nil {
			db.Event(eventDb, "upgrade").Trace(err, "Failed to upgrade db to version: %s", db.Pd.Version)
		}
	}()

	if new, _ := GetDb(db.GetName()); new != nil {
		db = new
	} else {
		err = storage.ErrNoNode
		return
	}

	if !db.Status.Available {
		err = ErrUnavailable
		return
	}

	if db.Status.UpgradeState == upgrading {
		logs.Warn("db %q is upgrading", db.GetName())
		return
	}

	// check all pods whether need to upgrade

	pods, err := k8sutil.GetPods(db.GetName(), "")
	if err != nil {
		return err
	}
	need := false
	for i := range pods {
		pod := pods[i]
		if needUpgrade(&pod, db.Pd.Version) {
			need = true
			break
		}
	}
	if !need {
		return nil
	}

	db.Status.UpgradeState = upgrading
	if err = db.update(); err != nil {
		return err
	}

	defer func() {
		st := upgradeOk
		if err != nil {
			st = upgradeFailed
		}
		db.Status.UpgradeState = st
		if uerr := db.update(); uerr != nil {
			logs.Error("failed to update db %q: %v", db.GetName(), uerr)
			if uerr != nil {
				err = uerr
			}
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
	return
}

func upgradeOne(name, image, version string) (bool, error) {
	pod, err := k8sutil.GetPod(name)
	if err != nil {
		return false, err
	}

	if !needUpgrade(pod, version) {
		return false, nil
	}

	logs.Info("start upgrading %q from %s to %s", name, k8sutil.GetTidbVersion(pod), version)

	if err = k8sutil.PatchPod(pod, waitPodRuningTimeout, func(np *v1.Pod) {
		np.Spec.Containers[0].Image = image
		k8sutil.SetTidbVersion(np, version)
	}); err != nil {
		return false, err
	}

	logs.Info("end upgrade", name)
	return true, nil
}

func upgradeRC(name, image, version string) error {
	err := k8sutil.PatchRc(name, func(rc *v1.ReplicationController) {
		logs.Info("start upgrading replicationcontroller(%s) from %s to %s", name, k8sutil.GetTidbVersion(rc), version)
		k8sutil.SetTidbVersion(rc, version)
		rc.Spec.Template.Spec.Containers[0].Image = image
	})
	logs.Info("end upgrade", name)
	return err
}

func needUpgrade(i interface{}, version string) bool {
	return k8sutil.GetTidbVersion(i) != version
}
