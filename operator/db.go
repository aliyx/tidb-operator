package operator

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
	tsql "github.com/ffan/tidb-operator/pkg/util/mysqlutil"

	"sync"
)

const (
	stopTidbTimeout                   = 60 // 60s
	waitPodRuningTimeout              = 90 * time.Second
	waitTidbComponentAvailableTimeout = 90 * time.Second

	// wait leader election and data sync
	pdUpgradeInterval = 15 * time.Second
	// wait leader election and data sync
	tikvUpgradeInterval = 60 * time.Second
	tidbUpgradeInterval = 15 * time.Second

	scaling      = 1 << 8
	tikvScaleErr = 1
	tidbScaleErr = 1 << 1

	// pd/tikv/tidb grace period is 5s, so +3s
	terminationGracePeriodSeconds = 8

	// test:3minute product:60minute
	tikvMaxDowntime = 3 * 60
	// ScaleUndefined no scale request
	ScaleUndefined int = iota
	// ScalePending wait for the admin to scale
	ScalePending
	// ScaleFailure scale failure
	ScaleFailure
	// Scaled scale success
	Scaled
)

var (
	// protect locks
	mu      sync.Mutex
	lockers = make(map[string]*sync.Mutex)
	doings  = make(map[string]struct{})
)

// Update patch db for external services
func (db *Db) Update(newDb *Db) (err error) {
	db.Operator = newDb.Operator
	switch newDb.Operator {
	// case "patch":
	// 	newDb.update()
	case "audit":
		if db.Status.Phase > PhaseUndefined {
			return ErrUnsupportPatch
		}
		switch newDb.Status.Phase {
		case PhaseRefuse:
			db.Status.Phase = PhaseRefuse
			db.Owner.Reason = newDb.Owner.Reason
			return db.update()
		case PhaseUndefined:
			db.Status.Phase = PhaseUndefined
			if err = db.update(); err != nil {
				return
			}
			go db.Install(true)
		default:
			return ErrUnsupportPatch
		}
	case "start":
		go db.Install(true)
	case "stop":
		go db.Uninstall(true)
	case "restart":
		go db.Reinstall()
	case "upgrade":
		if db.Pd.Version == newDb.Pd.Version {
			return
		}
		if newDb.Pd.Version != newDb.Tikv.Version || newDb.Tikv.Version != newDb.Tidb.Version {
			return fmt.Errorf("currently only support all versions are consistent")
		}
		db.Pd.Version = newDb.Pd.Version
		db.Tikv.Version = newDb.Tikv.Version
		db.Tidb.Version = newDb.Tidb.Version
		if err = db.update(); err != nil {
			return
		}
		go db.Reconcile()
	case "scale":
		if err := db.Tikv.checkScale(newDb.Tikv.Replicas); err != nil {
			return err
		}
		if err := db.Tidb.checkScale(newDb.Tidb.Replicas); err != nil {
			return err
		}
		db.Status.ScaleCount++
		db.Tikv.Replicas = newDb.Tikv.Replicas
		db.Tidb.Replicas = newDb.Tidb.Replicas
		if err = db.update(); err != nil {
			return
		}
		go db.Reconcile()
	case "syncMigrateStat":
		return db.SyncMigrateStat(newDb.Status.MigrateState, newDb.Status.Reason)
	default:
		return ErrUnsupportPatch
	}
	return
}

// Install tidb cluster and init user privileges
func (db *Db) Install(lock bool) (err error) {
	if lock {
		if !db.TryLock() {
			return
		}
		defer db.Unlock()
	}

	e := db.Event(eventDb, "install")
	defer func() {
		parseError(db, err)
		e.Trace(err, "Install tidb cluster on kubernetes")
		if err = db.update(); err != nil {
			db.Event(eventDb, "install").Trace(err, "Failed to update db")
		}
	}()

	if db.Status.Phase != PhaseUndefined {
		err = ErrRepeatOperation
		return err
	}

	logs.Info("start installing db", db.GetName())
	if err = db.Pd.install(); err != nil {
		return
	}
	if err = db.Tikv.install(); err != nil {
		return
	}
	if err = db.Tidb.install(); err != nil {
		return
	}
	logs.Info("wait 30s for tidb %q cluster to initialize", db.GetName())
	time.Sleep(30 * time.Second)
	if err = db.initSchema(); err != nil {
		return
	}
	logs.Info("end install db", db.GetName())
	return
}

// create user specify schema and set database privileges
func (db *Db) initSchema() (err error) {
	e := db.Event(eventDb, "init")
	defer func() {
		ph := PhaseTidbInited
		if err != nil {
			ph = PhaseTidbInitFailed
		} else {
			db.Status.Available = true
		}
		db.Status.Phase = ph
		e.Trace(err, fmt.Sprintf("Create schema %s and set database privileges", db.Schema.Name))
	}()

	if db.Status.Phase != PhaseTidbStarted {
		err = fmt.Errorf("tidb %q no started", db.GetName())
		return
	}

	var (
		h string
		p string
	)
	if h, p, err = net.SplitHostPort(db.Status.OuterAddresses[0]); err != nil {
		return
	}
	port, _ := strconv.Atoi(p)
	my := tsql.NewMysql(db.Schema.Name, h, port, db.Schema.User, db.Schema.Password)
	if err = my.CreateDatabaseAndGrant(); err != nil {
		return
	}
	return
}

// Uninstall tidb from kubernetes
func (db *Db) Uninstall(lock bool) (err error) {
	if lock {
		if !db.TryLock() {
			return
		}
		defer db.Unlock()
	}
	if db.Status.Phase < PhaseUndefined {
		return
	}

	db.Status.Available = false
	db.Status.Phase = PhaseTidbUninstalling
	if err = db.update(); err != nil {
		db.Event(eventDb, "uninstall").Trace(err, "Failed to update db")
		return err
	}

	logs.Info("start uninstalling db", db.GetName())
	e := db.Event(eventDb, "uninstall")
	defer func() {
		ph := PhaseUndefined
		if err == nil {
			if started(db.GetName()) {
				ph = PhaseTidbUninstalling
				err = fmt.Errorf("async delete pods timeout: %ds", stopTidbTimeout)
			} else {
				logs.Info("end uninstall db", db.GetName())
			}
		}
		db.Status.Phase = ph
		db.Status.Reason = ""
		db.Status.Message = ""
		db.Status.MigrateState = ""
		db.Status.UpgradeState = ""
		db.Status.ScaleState = 0
		db.Status.ScaleCount = 0
		db.Status.MigrateRetryCount = 0
		e.Trace(err, "Uninstall tidb cluster on k8s")
		if uerr := db.update(); uerr != nil {
			err = uerr
			db.Event(eventDb, "uninstall").Trace(err, "Failed to update db")
		}
	}()
	if err = db.StopMigrator(); err != nil {
		return
	}
	if err = db.Tidb.uninstall(); err != nil {
		return
	}
	if err = db.Tikv.uninstall(); err != nil {
		return
	}
	if err = db.Pd.uninstall(); err != nil {
		return
	}
	tries := int(stopTidbTimeout / 2)
	for i := 0; i < tries; i++ {
		if started(db.GetName()) {
			logs.Warn("db %q is not completely uninstalled yet", db.GetName())
			time.Sleep(2 * time.Second)
		} else {
			break
		}
	}
	return
}

// Reinstall first uninstall tidb, second install tidb
func (db *Db) Reinstall() (err error) {
	if !db.TryLock() {
		return
	}
	defer db.Unlock()

	e := db.Event(eventDb, "reinstall")
	defer func(ph Phase) {
		e.Trace(err, fmt.Sprintf("Reinstall db status from %d to %d", ph, db.Status.Phase))
	}(db.Status.Phase)

	if err = db.Uninstall(false); err != nil {
		return
	}

	if err = db.Install(false); err != nil {
		return
	}
	return
}

// NeedApproval whether the user creates tidb for approval
func NeedApproval(ID string, kvr, dbr uint) bool {
	if len(ID) < 1 {
		return true
	}
	dbs, err := GetDbs(ID)
	if err != nil {
		logs.Error("cann't get user %s db: %v", ID, err)
		return true
	}
	for _, db := range dbs {
		kvr = kvr + uint(db.Tikv.Replicas)
		dbr = dbr + uint(db.Tidb.Replicas)
	}
	md := getNonNullMetadata()
	if kvr > md.Spec.AC.KvReplicas || dbr > md.Spec.AC.DbReplicas {
		return true
	}
	return false
}

// Delete tidb
func Delete(cell string) error {
	var (
		err error
		db  *Db
	)
	if db, err = GetDb(cell); err != nil {
		return err
	}

	// async wait
	go func() {
		if !db.TryLock() {
			return
		}
		defer func() {
			defer db.Unlock()
			if err != nil {
				db.Event(eventDb, "delete").Trace(err, "Failed to delete db")
			}
		}()

		db.Operator = "stop"
		logs.Info("start deleting db", db.GetName())
		if err = db.Uninstall(false); err != nil {
			return
		}
		if err = delEventsBy(db.GetName()); err != nil {
			return
		}
		if err = db.delete(); err != nil {
			return
		}
		logs.Info("end delete db", db.GetName())
	}()
	return nil
}

func started(cell string) bool {
	pods, err := k8sutil.ListPodNames(cell, "")
	if err != nil {
		logs.Error("Get %s pods error: %v", cell, err)
	}
	return len(pods) > 0
}

// Locker get rwlocker
func (db *Db) Locker() *sync.Mutex {
	mu.Lock()
	defer mu.Unlock()
	rw, _ := lockers[db.GetName()]
	return rw
}

// TryLock try lock db
func (db *Db) TryLock() (locked bool) {
	rw := db.Locker()
	if rw != nil {
		rw.Lock()
		// double-check
		if n := db.Locker(); n != nil {
			if new, err := GetDb(db.GetName()); new != nil {
				db = new
				doings[db.GetName()] = struct{}{}
				locked = true
			} else {
				logs.Error("could get db %q: %", db.GetName(), err)
				locked = false
				rw.Unlock()
			}
		} else {
			locked = false
			rw.Unlock()
		}
	} else {
		locked = false
	}
	if !locked {
		logs.Error("could not try lock db", db.GetName())
	}
	return
}

// Unlock db
func (db *Db) Unlock() {
	rw := db.Locker()
	if rw == nil {
		panic(fmt.Sprintf("cann't get db %s locker", db.GetName()))
	}
	delete(doings, db.GetName())
	rw.Unlock()
}

// Doing whether some operations are being performed
func (db *Db) Doing() bool {
	mu.Lock()
	defer mu.Unlock()
	_, ok := doings[db.GetName()]
	return ok
}

// Recycle db
func (db *Db) Recycle() {
	mu.Lock()
	defer mu.Unlock()
	delete(lockers, db.GetName())
	delete(doings, db.GetName())
}
