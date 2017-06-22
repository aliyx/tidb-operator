package models

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/pkg/k8sutil"
	"github.com/ffan/tidb-k8s/pkg/storage"
	"github.com/ghodss/yaml"

	tsql "github.com/ffan/tidb-k8s/pkg/mysqlutil"
)

// Phase tidb runing status
type Phase int

const (
	// PhaseRefuse user apply create a tidb
	PhaseRefuse Phase = iota - 2
	// PhaseAuditing wait admin to auditing user apply
	PhaseAuditing
	// PhaseUndefined undefined
	PhaseUndefined
	// PhasePdPending pd pods is starting
	PhasePdPending
	// PhasePdStartFailed fail to start all pod pods
	PhasePdStartFailed
	// PhasePdStarted pd pods started
	PhasePdStarted
	// PhaseTikvPending tikv pods is starting
	PhaseTikvPending
	// PhaseTikvStartFailed fail to start all tikv pods
	PhaseTikvStartFailed
	// PhaseTikvStarted tikv pods started
	PhaseTikvStarted
	// PhaseTidbPending tidb pods is starting
	PhaseTidbPending
	// PhaseTidbStartFailed fail to start all tidb pods
	PhaseTidbStartFailed
	// PhaseTidbStarted tidb pods started
	PhaseTidbStarted
	// PhaseTidbInitFailed fail to init tidb schema and privilage
	PhaseTidbInitFailed
	// PhaseTidbInited tidb aviliable
	PhaseTidbInited
	// PhaseTidbDeleting being uninstall tidb
	PhaseTidbDeleting
)

const (
	migrating          = "Migrating"
	migStartMigrateErr = "StartMigrationTaskError"

	stopTidbTimeout                   = 60 // 60s
	waitPodRuningTimeout              = 60 * time.Second
	waitTidbComponentAvailableTimeout = 60 * time.Second

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

var (
	errNoInstalled    = errors.New("no installed")
	errInvalidReplica = errors.New("invalid replica")

	// ErrRepeatOperation is returned by functions to specify the operation is executing.
	ErrRepeatOperation = errors.New("the previous operation is being executed, please stop first")
)

func (db *Db) initSchema() (err error) {
	e := NewEvent(db.Metadata.Name, "db", "init")
	defer func() {
		ph := PhaseTidbInited
		if err != nil {
			ph = PhaseTidbInitFailed
		} else {
			db.Status.Available = true
		}
		db.Status.Phase = ph
		err = db.update()
		e.Trace(err, "Init database privileges")
	}()
	if !db.Tidb.isOk() {
		err = fmt.Errorf(`tidb "%s" no started`, db.Metadata.Name)
		return
	}
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
func Install(cell string, ch chan int) (err error) {
	var db *Db
	if db, err = GetDb(cell); err != nil {
		logs.Error("get db %s err: %v", cell, err)
		return err
	}
	if db.Status.Phase != PhaseUndefined && db.Status.Phase != PhaseAuditing {
		return ErrRepeatOperation
	}
	go func() {
		e := NewEvent(cell, "db", "install")
		defer func() {
			e.Trace(err, "Start installing tidb cluster on kubernete")
			ch <- 0
		}()
		if err = db.Pd.install(); err != nil {
			logs.Error("Install pd %s on k8s err: %v", cell, err)
			return
		}
		if err = db.Tikv.install(); err != nil {
			logs.Error("Install tikv %s on k8s err: %v", cell, err)
			return
		}
		if err = db.Tidb.install(); err != nil {
			logs.Error("Install tidb %s on k8s err: %v", cell, err)
			return
		}
		if err = db.initSchema(); err != nil {
			logs.Error("Init tidb %s privileges err: %v", cell, err)
			return
		}
	}()
	return nil
}

// Uninstall tidb
func Uninstall(cell string, ch chan int) (err error) {
	defer func() {
		if err != nil {
			if ch != nil {
				ch <- 0
			}
		}
	}()
	var db *Db
	if db, err = GetDb(cell); err != nil {
		return err
	}
	if db.Status.Phase <= PhaseUndefined {
		err = errNoInstalled
		return err
	}
	db.Status.Available = false
	db.Status.Phase = PhaseTidbDeleting
	if err = db.update(); err != nil {
		return err
	}
	// waiting for all pods deleted from k8s
	go func() {
		e := NewEvent(cell, "db", "uninstall")
		defer func() {
			stoped := 1
			ph := PhaseUndefined
			if started(cell) {
				ph = PhaseTidbDeleting
				stoped = 0
				err = errors.New("async delete pods timeout")
			}
			db.Status.Phase = ph
			if uerr := db.update(); err != nil {
				logs.Error("update db error: %", uerr)
			}
			e.Trace(err, "Uninstall db all pods/rc/service on k8s")
			if ch != nil {
				ch <- stoped
			}
		}()
		if err = stopMigrateTask(cell); err != nil {
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
			if started(cell) {
				logs.Warn(`tidb "%s" has not been cleared yet`, cell)
				time.Sleep(2 * time.Second)
			} else {
				break
			}
		}
	}()
	return err
}

// Reinstall first uninstall tidb, second install tidb
func Reinstall(cell string) (err error) {
	db, err := GetDb(cell)
	if err != nil {
		return err
	}
	go func() {
		e := NewEvent(cell, "db", "restart")
		defer func(ph Phase) {
			e.Trace(err, fmt.Sprintf("Restart db status from %d -> %d", ph, db.Status.Phase))
		}(db.Status.Phase)
		ch := make(chan int, 1)
		if err = Uninstall(cell, ch); err != nil {
			logs.Error("delete db %s error: %v", cell, err)
			return
		}
		// waiting for all pod deleted
		stoped := <-ch
		if stoped == 0 {
			logs.Error("Uninstall db %s timeout", cell)
			return
		}
		if err = Install(cell, ch); err != nil {
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
	if err := my.Check(); err != nil {
		return fmt.Errorf(`schema "%s" does not support migration error: %v`, db.Metadata.Name, err)
	}
	db.Status.MigrateState = migrating
	if err := db.update(); err != nil {
		return err
	}
	return db.startMigrateTask(my)
}

// UpdateMigrateStat update tidb migrate stat
func (db *Db) UpdateMigrateStat(s, desc string) (err error) {
	var e *Event
	db.Status.MigrateState = s
	if err := db.update(); err != nil {
		return err
	}
	logs.Info("Current tidb %s migrate status: %s", db.Metadata.Name, s)
	switch s {
	case "Dumping":
		e = NewEvent(db.Metadata.Name, "migration", "dump")
		e.Trace(nil, "Start Dumping mysql data to local")
	case "DumpError":
		e = NewEvent(db.Metadata.Name, "migration", "dump")
		e.Trace(fmt.Errorf("Unknow"), "Dumped mysql data to local error")
	case "Loading":
		e = NewEvent(db.Metadata.Name, "migration", "load")
		e.Trace(nil, "End dumped and start loading local to tidb")
	case "LoadError":
		e = NewEvent(db.Metadata.Name, "migration", "load")
		e.Trace(fmt.Errorf("Unknow"), "Loaded local data to tidb error")
	case "Finished":
		e = NewEvent(db.Metadata.Name, "tidb", "migration")
		err = stopMigrateTask(db.Metadata.Name)
		e.Trace(err, "End the full migration and delete migration docker on k8s")
	case "Syncing":
		e = NewEvent(db.Metadata.Name, "migration", "sync")
		e.Trace(nil, "Finished load and start incremental syncing mysql data to tidb")
	}
	return nil
}

func (db *Db) startMigrateTask(my *tsql.Migration) (err error) {
	sync := ""
	if my.ToggleSync {
		sync = "sync"
	}
	r := strings.NewReplacer(
		"{{namespace}}", getNamespace(),
		"{{cell}}", db.Metadata.Name,
		"{{image}}", fmt.Sprintf("%s/migration:latest", imageRegistry),
		"{{sh}}", my.Src.IP, "{{sP}}", fmt.Sprintf("%v", my.Src.Port),
		"{{su}}", my.Src.User, "{{sp}}", my.Src.Password,
		"{{db}}", my.Src.Database,
		"{{dh}}", my.Dest.IP, "{{dP}}", fmt.Sprintf("%v", my.Dest.Port),
		"{{duser}}", my.Dest.User, "{{dp}}", my.Dest.Password,
		"{{sync}}", sync,
		"{{api}}", my.NotifyAPI)
	s := r.Replace(mysqlMigrateYaml)
	var j []byte
	if j, err = yaml.YAMLToJSON([]byte(s)); err != nil {
		return err
	}
	go func() {
		e := NewEvent(db.Metadata.Name, "tidb", "migration")
		defer func() {
			e.Trace(err, "Startup migration docker on k8s")
		}()
		if _, err = k8sutil.CreateAndWaitPodByJSON(j, waitPodRuningTimeout); err != nil {
			db.Status.MigrateState = migStartMigrateErr
			err = db.update()
			return
		}
	}()
	return nil
}

func stopMigrateTask(cell string) error {
	return k8sutil.DeletePodsBy(cell, "migration")
}

// Scale tikv and tidb
func Scale(cell string, kvReplica, dbReplica int) (err error) {
	var db *Db
	if db, err = GetDb(cell); err != nil {
		return err
	}
	if !db.Status.Available {
		return fmt.Errorf("tidb %s unavailable", cell)
	}
	if db.Status.ScaleState&scaling > 0 {
		return fmt.Errorf("tidb %s is scaling", cell)
	}
	db.Status.ScaleState |= scaling
	db.update()
	var wg sync.WaitGroup
	db.scaleTikvs(kvReplica, &wg)
	db.scaleTidbs(dbReplica, &wg)
	go func() {
		wg.Wait()
		db.Status.ScaleState ^= scaling
		db.update()
	}()
	return nil
}

// NeedLimitResources whether the user creates tidb for approval
func NeedLimitResources(ID string, kvr, dbr uint) bool {
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

// Delete tidb from k8s
func (db *Db) Delete(callbacks ...clear) (err error) {
	if len(db.Metadata.Name) < 1 {
		return nil
	}
	ch := make(chan int, 1)
	if err = Uninstall(db.Metadata.Name, ch); err != nil && err != errNoInstalled {
		return err
	}
	if err = delEventsBy(db.Metadata.Name); err != nil {
		return err
	}
	go func() {
		db.Status.Phase = PhaseTidbDeleting
		db.update()
		// wait end
		<-ch
		for {
			if !started(db.Metadata.Name) {
				if err := db.delete(); err != nil && err != storage.ErrNoNode {
					logs.Error("delete tidb error: %v", err)
					return
				}
				if len(callbacks) > 0 {
					for _, call := range callbacks {
						call()
					}
				}
				return
			}
			time.Sleep(time.Second)
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
