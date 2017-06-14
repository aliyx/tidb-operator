package models

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/pkg/k8sutil"
	"github.com/ghodss/yaml"

	tsql "github.com/ffan/tidb-k8s/pkg/mysqlutil"
)

var (
	errNoInstalled = errors.New("no installed")
)

func (db *Tidb) initSchema() (err error) {
	e := NewEvent(db.Cell, "tidb", "init")
	defer func() {
		ph := tidbInited
		if err != nil {
			ph = tidbInitFailed
		} else {
			db.Status.Available = true
		}
		db.Status.Phase = ph
		err = db.update()
		e.Trace(err, "Init tidb privileges")
	}()
	if !db.isOk() {
		err = fmt.Errorf(`tidb "%s" no started`, db.Cell)
		return
	}
	var (
		h string
		p string
	)
	if h, p, err = net.SplitHostPort(db.OuterAddresses[0]); err != nil {
		return err
	}
	port, _ := strconv.Atoi(p)
	for _, schema := range db.Schemas {
		my := tsql.NewMysql(schema.Name, h, port, schema.User, schema.Password)
		if err = my.Init(); err != nil {
			return err
		}
	}
	return nil
}

// Install tidb
func Install(cell string, ch chan int) (err error) {
	var db *Tidb
	if db, err = GetTidb(cell); err != nil {
		logs.Error("get tidb %s err: %v", cell, err)
		return err
	}
	if db.Status.Phase != Undefined {
		return ErrRepeatOperation
	}
	go func() {
		e := NewEvent(cell, "tidb", "install")
		defer func() {
			e.Trace(err, "Start installing tidb cluster on kubernete")
			ch <- 0
		}()
		if err = db.Pd.install(); err != nil {
			logs.Error("Start pd %s on k8s err: %v", cell, err)
			return
		}
		if err = db.Tikv.install(); err != nil {
			logs.Error("Start tikv %s on k8s err: %v", cell, err)
			return
		}
		if err = db.install(); err != nil {
			logs.Error("Start tidb %s on k8s err: %v", cell, err)
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
	if !started(cell) {
		return errNoInstalled
	}
	var db *Tidb
	if db, err = GetTidb(cell); err != nil {
		return err
	}
	db.Status.Available = false
	if err = db.update(); err != nil {
		return err
	}
	// waiting for all pods deleted from k8s
	go func() {
		e := NewEvent(cell, "db", "uninstall")
		defer func() {
			stoped := 1
			ph := Undefined
			if started(cell) {
				ph = tidbStopFailed
				stoped = 0
				err = errors.New("async delete pods timeout")
			}
			db.Status.Phase = ph
			err = db.update()
			if ch != nil {
				ch <- stoped
			}
			e.Trace(err, "Uninstall tidb pods/rc/service on k8s")
		}()
		if err = stopMigrateTask(cell); err != nil {
			return
		}
		if err = db.uninstall(); err != nil {
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
	db, err := GetTidb(cell)
	if err != nil {
		return err
	}
	go func() {
		e := NewEvent(cell, "tidb", "restart")
		defer func(ph Phase) {
			e.Trace(err, fmt.Sprintf("Restart tidb status from %d -> %d", ph, db.Status.Phase))
		}(db.Status.Phase)
		ch := make(chan int, 1)
		if err = Uninstall(cell, ch); err != nil {
			logs.Error("delete tidb %s error: %v", cell, err)
			return
		}
		// waiting for all pod deleted
		stoped := <-ch
		if stoped == 0 {
			logs.Error("Uninstall tidb %s timeout", cell)
			return
		}
		if err = Install(cell, ch); err != nil {
			logs.Error("Install tidb %s error: %v", cell, err)
			return
		}
		// end
		<-ch
	}()
	return nil
}

// Migrate the mysql data to the current tidb
func (db *Tidb) Migrate(src tsql.Mysql, notify string, sync bool) error {
	if !db.Status.Available {
		return fmt.Errorf("tidb is not available")
	}
	// if db.MigrateState != "" {
	// 	return errors.New("can not migrate multiple times")
	// }
	if len(src.IP) < 1 || src.Port < 1 || len(src.User) < 1 || len(src.Password) < 1 || len(src.Database) < 1 {
		return fmt.Errorf("invalid database %+v", src)
	}
	var sch *Schema
	for _, s := range db.Schemas {
		if s.Name == src.Database {
			sch = &s
		}
	}
	if sch == nil {
		return fmt.Errorf("both schemas must be the same")
	}
	h, p, err := net.SplitHostPort(db.OuterAddresses[0])
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
		return fmt.Errorf(`schema "%s" does not support migration error: %v`, db.Cell, err)
	}
	db.Status.MigrateState = migrating
	if err := db.update(); err != nil {
		return err
	}
	return db.startMigrateTask(my)
}

// UpdateMigrateStat update tidb migrate stat
func (db *Tidb) UpdateMigrateStat(s, desc string) (err error) {
	var e *Event
	db.Status.MigrateState = s
	if err := db.update(); err != nil {
		return err
	}
	logs.Info("Current tidb %s migrate status: %s", db.Cell, s)
	switch s {
	case "Dumping":
		e = NewEvent(db.Cell, "migration", "dump")
		e.Trace(nil, "Start Dumping mysql data to local")
	case "DumpError":
		e = NewEvent(db.Cell, "migration", "dump")
		e.Trace(fmt.Errorf("Unknow"), "Dumped mysql data to local error")
	case "Loading":
		e = NewEvent(db.Cell, "migration", "load")
		e.Trace(nil, "End dumped and start loading local to tidb")
	case "LoadError":
		e = NewEvent(db.Cell, "migration", "load")
		e.Trace(fmt.Errorf("Unknow"), "Loaded local data to tidb error")
	case "Finished":
		e = NewEvent(db.Cell, "tidb", "migration")
		err = stopMigrateTask(db.Cell)
		e.Trace(err, "End the full migration and delete migration docker on k8s")
	case "Syncing":
		e = NewEvent(db.Cell, "migration", "sync")
		e.Trace(nil, "Finished load and start incremental syncing mysql data to tidb")
	}
	return nil
}

func (db *Tidb) startMigrateTask(my *tsql.Migration) (err error) {
	sync := ""
	if my.ToggleSync {
		sync = "sync"
	}
	r := strings.NewReplacer(
		"{{namespace}}", getNamespace(),
		"{{cell}}", db.Cell,
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
		e := NewEvent(db.Cell, "tidb", "migration")
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
