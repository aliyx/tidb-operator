package operator

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/storage"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
	"github.com/ghodss/yaml"

	"sync"

	tsql "github.com/ffan/tidb-operator/pkg/util/mysqlutil"
)

const (
	migrating          = "Migrating"
	migStartMigrateErr = "StartMigrationTaskError"

	stopTidbTimeout                   = 60 // 60s
	waitPodRuningTimeout              = 180 * time.Second
	waitTidbComponentAvailableTimeout = 180 * time.Second

	// wait leader election
	upgradeInterval = 15 * time.Second

	scaling      = 1 << 8
	tikvScaleErr = 1
	tidbScaleErr = 1 << 1

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

// create user specify schema and set database privileges
func (db *Db) initSchema() (err error) {
	if db.Status.Phase != PhaseTidbStarted {
		return fmt.Errorf("tidb '%s' no started", db.GetName())
	}

	// save savepoint
	if err = db.update(); err != nil {
		return nil
	}

	e := NewEvent(db.GetName(), "db", "init")
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

	var (
		h string
		p string
	)
	if h, p, err = net.SplitHostPort(db.Status.OuterAddresses[0]); err != nil {
		return err
	}
	port, _ := strconv.Atoi(p)
	my := tsql.NewMysql(db.Schema.Name, h, port, db.Schema.User, db.Schema.Password)
	if err = my.CreateDatabaseAndGrant(); err != nil {
		return err
	}
	return nil
}

// Install tidb
func (db *Db) Install(ch chan int) (err error) {
	// check status
	if db.Status.Phase < PhaseUndefined {
		return fmt.Errorf("db %s may be in the approval or no passed", db.GetName())
	}
	if db.Status.Phase != PhaseUndefined {
		return ErrRepeatOperation
	}

	go func() {
		if !db.TryLock() {
			return
		}
		defer db.Unlock()
		// double-check
		if new, _ := GetDb(db.GetName()); new == nil || new.Status.Phase != PhaseUndefined {
			logs.Error("db %s was modified before install", db.GetName())
			return
		}

		e := NewEvent(db.GetName(), "db", "install")
		defer func() {
			parseError(db, err)
			if err != nil {
				logs.Error("failed to install db %s on k8s: %v", db.GetName(), err)
			}
			e.Trace(err, "Start installing tidb cluster on kubernetes")

			if err = db.update(); err != nil {
				logs.Error("failed to update db %s: %v", db.GetName(), err)
			}
			if err != nil {
				ch <- 1
			} else {
				ch <- 0
			}
		}()
		if err = db.Pd.install(); err != nil {
			return
		}
		if err = db.Tikv.install(); err != nil {
			return
		}
		if err = db.Tidb.install(); err != nil {
			return
		}
		if err = db.initSchema(); err != nil {
			return
		}
	}()
	return nil
}

// Uninstall tidb from kubernetes
func (db *Db) Uninstall(ch chan int) (err error) {
	if db.Status.Phase < PhaseUndefined {
		if ch != nil {
			ch <- 0
		}
		return nil
	}
	db.Status.Available = false
	db.Status.Phase = PhaseTidbUninstalling
	if err = db.update(); err != nil {
		if ch != nil {
			ch <- 1
		}
		return err
	}
	// aync waiting for all pods deleted from k8s
	go func() {
		if !db.TryLock() {
			return
		}
		defer db.Unlock()
		// double-check
		if new, _ := GetDb(db.GetName()); new == nil || new.Status.Phase != PhaseTidbUninstalling {
			logs.Error("db %s was modified before uninstall", db.GetName())
			return
		}
		logs.Info("start uninstall db ", db.GetName())
		e := NewEvent(db.GetName(), "db", "uninstall")
		defer func() {
			stoped := 0
			ph := PhaseUndefined
			if started(db.GetName()) {
				ph = PhaseTidbUninstalling
				stoped = 1
				err = fmt.Errorf("async delete pods timeout: %v", err)
			}
			db.Status.Phase = ph
			db.Status.Reason = ""
			db.Status.Message = ""
			db.Status.MigrateState = ""
			db.Status.UpgradeState = ""
			db.Status.ScaleState = 0
			db.Status.ScaleCount = 0
			if uerr := db.update(); uerr != nil {
				logs.Error("failed to update db %s: %v", db.GetName(), uerr)
			}
			e.Trace(err, "Uninstall tidb all pods/rc/service components on k8s")
			if ch != nil {
				ch <- stoped
			}
		}()
		if err = db.stopMigrator(); err != nil {
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
		for i := 0; i < int(stopTidbTimeout/2); i++ {
			if started(db.GetName()) {
				logs.Warn("db '%s' is not completely uninstalled yet", db.GetName())
				time.Sleep(2 * time.Second)
			} else {
				break
			}
		}
	}()
	return err
}

// Reinstall first uninstall tidb, second install tidb
func (db *Db) Reinstall(cell string) (err error) {
	go func() {
		e := NewEvent(cell, "db", "reinstall")
		defer func(ph Phase) {
			e.Trace(err, fmt.Sprintf("Reinstall db status from %d to %d", ph, db.Status.Phase))
		}(db.Status.Phase)

		ch := make(chan int, 1)
		if err = db.Uninstall(ch); err != nil {
			logs.Error("delete db %s error: %v", cell, err)
			return
		}
		// waiting for all pod deleted
		stoped := <-ch
		if stoped != 0 {
			logs.Error("Uninstall db %s timeout", cell)
			return
		}

		if err = db.Install(ch); err != nil {
			logs.Error("Install db %s error: %v", cell, err)
			return
		}
		// end
		<-ch
	}()
	return nil
}

// Migrate the mysql data to the current tidb
func (db *Db) Migrate(src tsql.Mysql, notify string, sync bool) error {
	if !db.Status.Available {
		return fmt.Errorf("tidb is not available")
	}
	// if db.MigrateState != "" {
	// 	return errors.New("can not migrate multiple times")
	// }
	if len(src.IP) < 1 || src.Port < 1 || len(src.User) < 1 || len(src.Password) < 1 || len(src.Database) < 1 {
		return fmt.Errorf("invalid database %+v", src)
	}
	if db.Schema.Name != src.Database {
		return fmt.Errorf("both schemas must be the same")
	}
	sch := db.Schema
	h, p, err := net.SplitHostPort(db.Status.OuterAddresses[0])
	if err != nil {
		return err
	}
	port, _ := strconv.Atoi(p)
	my := &tsql.Migration{
		Src:  src,
		Dest: *tsql.NewMysql(sch.Name, h, port, sch.User, sch.Password),

		ToggleSync: sync,
		NotifyAPI:  notify,
	}
	logs.Debug("migrator object: %v", my)
	if err := my.Check(); err != nil {
		return fmt.Errorf(`schema "%s" does not support migration error: %v`, db.Metadata.Name, err)
	}
	db.Status.MigrateState = migrating
	if err := db.update(); err != nil {
		return err
	}
	return db.startMigrator(my)
}

// SyncMigrateStat update tidb migrate stat
func (db *Db) SyncMigrateStat() (err error) {
	var e *Event
	if err := db.update(); err != nil {
		return err
	}
	logs.Info("Current tidb %s migrate status: %s", db.GetName(), db.Status.MigrateState)
	switch db.Status.MigrateState {
	case "Finished":
		e = NewEvent(db.GetName(), "db/migrator", "stop")
		err = db.stopMigrator()
		e.Trace(err, "End the full migrate and delete migrator from k8s")
	case "Syncing":
		e = NewEvent(db.GetName(), "db/migrator", "sync")
		e.Trace(nil, "Finished load and start incremental syncing mysql data to tidb")
	default:
		return fmt.Errorf("unknow status")
	}
	return nil
}

func (db *Db) startMigrator(my *tsql.Migration) (err error) {
	sync := "load"
	if my.ToggleSync {
		sync = "sync"
	}
	r := strings.NewReplacer(
		"{{namespace}}", getNamespace(),
		"{{cell}}", db.GetName(),
		"{{image}}", fmt.Sprintf("%s/migrator:latest", imageRegistry),
		"{{sh}}", my.Src.IP, "{{sP}}", fmt.Sprintf("%v", my.Src.Port),
		"{{su}}", my.Src.User, "{{sp}}", my.Src.Password,
		"{{db}}", my.Src.Database,
		"{{dh}}", my.Dest.IP, "{{dP}}", fmt.Sprintf("%v", my.Dest.Port),
		"{{du}}", my.Dest.User, "{{dp}}", my.Dest.Password,
		"{{op}}", sync,
		"{{api}}", my.NotifyAPI)
	s := r.Replace(mysqlMigrateYaml)
	var j []byte
	if j, err = yaml.YAMLToJSON([]byte(s)); err != nil {
		return err
	}

	go func() {
		e := NewEvent(db.GetName(), "db/migrator", "start")
		defer func() {
			e.Trace(err, "Startup migrator on k8s")
		}()
		if _, err = k8sutil.CreateAndWaitJobByJSON(j, waitPodRuningTimeout); err != nil {
			db.Status.MigrateState = migStartMigrateErr
			if uerr := db.update(); uerr != nil {
				logs.Error("failed to update db %s error: %v", db.GetName(), uerr)
			}
			return
		}
	}()

	return nil
}

func (db *Db) stopMigrator() error {
	return k8sutil.DeleteJob("migrator-" + db.GetName())
}

// Scale tikv and tidb
func (db *Db) Scale(kvReplica, dbReplica int) (err error) {
	if !db.Status.Available {
		return ErrUnavailable
	}
	if db.Status.ScaleState&scaling > 0 {
		return ErrScaling
	}
	db.Status.ScaleState |= scaling
	if err = db.update(); err != nil {
		return err
	}

	go func() {
		if !db.TryLock() {
			return
		}
		defer db.Unlock()
		// double-check
		if new, _ := GetDb(db.GetName()); new == nil || !new.Status.Available || db.Status.ScaleState&scaling == 0 ||
			db.Tikv.Replicas != new.Tikv.Replicas || db.Tidb.Replicas != new.Tidb.Replicas {
			logs.Error("db %s was modified before scale", db.GetName())
			return
		}

		defer func() {
			parseError(db, err)
			db.Status.ScaleState ^= scaling
			if err = db.update(); err != nil {
				logs.Error("failed to update db %s: %v", db.GetName(), err)
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

	}()
	return nil
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
		kvr = kvr + uint(db.Tikv.Spec.Replicas)
		dbr = dbr + uint(db.Tidb.Spec.Replicas)
	}
	md := getCachedMetadata()
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
		db.TryLock()
		defer db.Recycle()

		ch := make(chan int, 1)
		if err = db.Uninstall(ch); err != nil {
			return
		}

		// wait uninstalled
		stoped := <-ch
		if stoped != 0 {
			// fail to uninstall tidb, so quit
			return
		}
		if err = db.delete(); err != nil && err != storage.ErrNoNode {
			logs.Error("delete tidb error: %v", err)
			return
		}
		if err = delEventsBy(db.Metadata.Name); err != nil {
			logs.Error("delete event error: %v", err)
			return
		}
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
			doings[db.GetName()] = struct{}{}
			locked = true
		} else {
			locked = false
			rw.Unlock()
		}
	} else {
		locked = false
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
