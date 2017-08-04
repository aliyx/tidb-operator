package operator

import (
	"fmt"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
	"k8s.io/client-go/pkg/api/v1"
)

// Reconcile tikv and tidb desired status
// 1. reconcile replica count
// 2. reconcile version
func (db *Db) Reconcile(kvReplica, dbReplica int) (err error) {
	if !db.Status.Available {
		return ErrUnavailable
	}

	// check is scaling
	if db.Status.ScaleState&scaling > 0 {
		return ErrScaling
	}

	go func() {
		if !db.TryLock() {
			logs.Error("could not try lock db", db.GetName())
			return
		}
		defer db.Unlock()
		// double-check
		if new, _ := GetDb(db.GetName()); new == nil || !new.Status.Available || new.Status.ScaleState&scaling > 0 {
			logs.Error("db %q was modified before scale", db.GetName())
			return
		}

		logs.Debug("start reconciling db", db.GetName())
		db.Status.ScaleState |= scaling
		if err = db.update(); err != nil {
			logs.Error("failed to update db %q: %v", db.GetName(), err)
			return
		}

		defer func() {
			if err != nil {
				logs.Error("failed to reconcile tidb cluster %q, %v", db.GetName(), err)
			}
			parseError(db, err)
			db.Status.ScaleState ^= scaling
			if err = db.update(); err != nil {
				logs.Error("failed to update db %s: %v", db.GetName(), err)
			} else {
				logs.Debug("end reconciling db", db.GetName())
			}
		}()

		if err = db.reconcilePds(); err != nil {
			return
		}
		if err = db.reconcileTikvs(kvReplica); err != nil {
			return
		}
		if err = db.reconcileTidbs(dbReplica); err != nil {
			return
		}

		// check version
		err = db.Upgrade()
	}()
	return nil
}

// Upgrade tidb version
func (db *Db) Upgrade() error {
	if db.Status.UpgradeState == upgrading {
		return fmt.Errorf("db %q is upgrading", db.GetName())
	}

	// check all pod whether need to upgrade

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

	go func() {
		if !db.TryLock() {
			return
		}
		defer db.Unlock()
		// double-check
		if new, _ := GetDb(db.GetName()); new == nil || new.Status.UpgradeState == upgrading {
			logs.Error("db %q was modified before upgrade", db.GetName())
			return
		}
		db.Status.UpgradeState = upgrading
		if err = db.update(); err != nil {
			logs.Error("failed to update db %q: %v", db.GetName(), err)
			return
		}

		defer func() {
			st := upgradeOk
			if err != nil {
				st = upgradeFailed
				logs.Error("failed to upgrade db %q: %v", db.GetName(), err)
			}
			db.Status.UpgradeState = st
			if err = db.update(); err != nil {
				logs.Error("failed to update db %q: %v", db.GetName(), err)
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
