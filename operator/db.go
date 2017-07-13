package operator

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/storage"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
	"github.com/ghodss/yaml"

	tsql "github.com/ffan/tidb-operator/pkg/util/mysqlutil"
)

const (
	migrating          = "Migrating"
	migStartMigrateErr = "StartMigrationTaskError"

	stopTidbTimeout                   = 60 // 60s
	waitPodRuningTimeout              = 180 * time.Second
	waitTidbComponentAvailableTimeout = 180 * time.Second

	scaling      = 1 << 8
	tikvScaleErr = 1
	tidbScaleErr = 1 << 1
)

const (
	// ScaleUndefined no scale request
	ScaleUndefined int = iota
	// ScalePending wait for the admin to scale
	ScalePending
	// ScaleFailure scale failure
	ScaleFailure
	// Scaled scale success
	Scaled
)

// create user specify schema and set database privileges
func (db *Db) initSchema() (err error) {
	if db.Status.Phase != PhaseTidbStarted {
		return fmt.Errorf("tidb '%s' no started", db.GetName())
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
		if uerr := db.update(); uerr != nil {
			logs.Error("failed to update db %s: %v", db.GetName(), uerr)
		}
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
	// Startup means passing the audit
	if db.Status.Phase == PhaseAuditing {
		db.Status.Phase = PhaseUndefined
	}
	if db.Status.Phase < PhaseUndefined {
		return fmt.Errorf("db %s may be in the approval or no passed", db.GetName())
	}
	if db.Status.Phase != PhaseUndefined {
		return ErrRepeatOperation
	}

	go func() {
		hook.Add(1)
		defer hook.Done()

		e := NewEvent(db.GetName(), "db", "install")
		defer func() {
			e.Trace(err, "Start installing tidb cluster on kubernete")
			if err != nil {
				ch <- 1
			} else {
				ch <- 0
			}
		}()
		if err = db.Pd.install(); err != nil {
			logs.Error("failed to install pd %s on k8s: %v", db.GetName(), err)
			return
		}
		if err = db.Tikv.install(); err != nil {
			logs.Error("failed to install tikv %s on k8s: %v", db.GetName(), err)
			return
		}
		if err = db.Tidb.install(); err != nil {
			logs.Error("failed to install tidb %s on k8s: %v", db.GetName(), err)
			return
		}
		if err = db.initSchema(); err != nil {
			logs.Error("failed to init db %s privileges: %v", db.GetName(), err)
			return
		}
	}()
	return nil
}

// Uninstall tidb from kubernetes
func (db *Db) Uninstall(ch chan int) (err error) {
	if db.Status.Phase <= PhaseUndefined {
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
		hook.Add(1)
		defer hook.Done()

		e := NewEvent(db.GetName(), "db", "uninstall")
		defer func() {
			stoped := 0
			ph := PhaseUndefined
			if started(db.GetName()) {
				ph = PhaseTidbUninstalling
				stoped = 1
				err = errors.New("async delete pods timeout")
			}
			db.Status.Phase = ph
			if uerr := db.update(); err != nil {
				logs.Error("update db error: %", uerr)
			}
			e.Trace(err, "Uninstall tidb all pods/rc/service components on k8s")
			if ch != nil {
				ch <- stoped
			}
		}()
		if err = stopMigrateTask(db.GetName()); err != nil {
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
				logs.Warn(`tidb "%s" has not been cleared yet`, db.GetName())
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
		e := NewEvent(cell, "db", "restart")
		defer func(ph Phase) {
			e.Trace(err, fmt.Sprintf("Restart db status from %d -> %d", ph, db.Status.Phase))
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
	return db.startMigrateTask(my)
}

// SyncMigrateStat update tidb migrate stat
func (db *Db) SyncMigrateStat() (err error) {
	var e *Event
	if err := db.update(); err != nil {
		return err
	}
	logs.Info("Current tidb %s migrate status: %s", db.Metadata.Name, db.Status.MigrateState)
	switch db.Status.MigrateState {
	case "Finished":
		e = NewEvent(db.Metadata.Name, "db/migrator", "stop")
		err = stopMigrateTask(db.Metadata.Name)
		e.Trace(err, "End the full migrate and delete migrator from k8s")
	case "Syncing":
		e = NewEvent(db.Metadata.Name, "db/migrator", "sync")
		e.Trace(nil, "Finished load and start incremental syncing mysql data to tidb")
	default:
		return fmt.Errorf("unknow status")
	}
	return nil
}

func (db *Db) startMigrateTask(my *tsql.Migration) (err error) {
	sync := "load"
	if my.ToggleSync {
		sync = "sync"
	}
	r := strings.NewReplacer(
		"{{namespace}}", getNamespace(),
		"{{cell}}", db.Metadata.Name,
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
		e := NewEvent(db.Metadata.Name, "db/migrator", "start")
		defer func() {
			e.Trace(err, "Startup migrator on k8s")
		}()
		if _, err = k8sutil.CreateAndWaitPodByJSON(j, waitPodRuningTimeout); err != nil {
			db.Status.MigrateState = migStartMigrateErr
			if uerr := db.update(); uerr != nil {
				logs.Error("failed to update db %s error: %v", db.Metadata.Name, uerr)
			}
			return
		}
	}()

	return nil
}

func stopMigrateTask(cell string) error {
	return k8sutil.DeletePodsBy(cell, "migrator")
}

// Scale tikv and tidb
func (db *Db) Scale(kvReplica, dbReplica int) (err error) {
	hook.Add(1)
	defer hook.Done()

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
	var wg sync.WaitGroup
	db.scaleTikvs(kvReplica, &wg)
	db.scaleTidbs(dbReplica, &wg)
	go func() {
		wg.Wait()
		db.Status.ScaleState ^= scaling
		if err = db.update(); err != nil {
			logs.Error("failed to update db %s %v", db.GetName(), err)
		}
	}()
	return nil
}

// Limit whether the user creates tidb for approval
func Limit(ID string, kvr, dbr uint) bool {
	if len(ID) < 1 {
		return true
	}
	dbs, err := GetDbs(ID)
	if err != nil {
		logs.Error("cant get user %s dbs: %v", ID, err)
	}
	for _, db := range dbs {
		kvr = kvr + uint(db.Tikv.Spec.Replicas)
		dbr = dbr + uint(db.Tidb.Spec.Replicas)
	}
	md := getCachedMetadata()
	if kvr > md.Spec.AC.KvReplicas {
		return true
	}
	if dbr > md.Spec.AC.DbReplicas {
		return true
	}
	return false
}

type clear func()

// Delete tidb
func Delete(cell string) error {
	var (
		err error
		db  *Db
	)
	if db, err = GetDb(cell); err != nil {
		return err
	}
	ch := make(chan int, 1)
	if err = db.Uninstall(ch); err != nil {
		return err
	}

	// async wait
	go func() {
		// wait uninstalled
		stoped := <-ch
		if stoped != 0 {
			// fail to uninstall tidb, so quit
			return
		}
		if err := db.delete(); err != nil && err != storage.ErrNoNode {
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
		logs.Warn("Get %s pods error: %v", cell, err)
	}
	return len(pods) > 0
}
