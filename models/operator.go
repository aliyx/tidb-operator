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
		return fmt.Errorf(`tidb "%s" no started`, db.Cell)
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

// Start tidb server
func Start(cell string) (err error) {
	if started(cell) {
		return ErrRepeatOperation
	}
	var db *Tidb
	if db, err = GetTidb(cell); err != nil {
		logs.Error("Get tidb %s err: %v", cell, err)
		return err
	}
	go func() {
		e := NewEvent(cell, "tidb", "install")
		defer func() {
			e.Trace(err, "Start installing tidb cluster on kubernete")
		}()
		if err = db.Pd.install(); err != nil {
			logs.Error("Run pd %s on k8s err: %v", cell, err)
			return
		}
		if err = db.Tikv.install(); err != nil {
			logs.Error("Run tikv %s on k8s err: %v", cell, err)
			return
		}
		if err = db.install(); err != nil {
			logs.Error("Run tidb %s on k8s err: %v", cell, err)
			return
		}
		if err = db.initSchema(); err != nil {
			logs.Error("Init tidb %s privileges err: %v", cell, err)
			return
		}
	}()
	return nil
}

// Stop tidb server
func Stop(cell string, ch chan int) (err error) {
	// if !started(cell) {
	// 	return err
	// }
	var db *Tidb
	if db, err = GetTidb(cell); err != nil {
		return err
	}
	logs.Debug("%v", db)
	e := NewEvent(cell, "db", "uninstall")
	defer func() {
		if err != nil {
			logs.Error("uninstall tidb %s: %v", cell, err)
			e.Trace(err, "Uninstall tidb")
		}
	}()
	if err = stopMigrateTask(cell); err != nil {
		return err
	}
	if err = db.uninstall(); err != nil {
		return err
	}
	if err = db.Tikv.uninstall(); err != nil {
		return err
	}
	if err = db.Pd.uninstall(); err != nil {
		return err
	}
	// waiting for all pods deleted from k8s
	go func() {
		defer func() {
			stoped := 1
			st := Undefined
			if started(cell) {
				st = tidbStopFailed
				stoped = 0
				err = errors.New("async delete pods timeout")
			}
			rollout(cell, st)
			if ch != nil {
				ch <- stoped
			}
			e.Trace(err, "Stop tidb pods on k8s")
		}()
		for i := 0; i < defaultStopTidbTimeout; i++ {
			if started(cell) {
				logs.Warn(`tidb "%s" has not been cleared yet`, cell)
				time.Sleep(time.Second)
			} else {
				break
			}
		}
	}()
	return err
}

// Restart first stop tidb, second start tidb
func Restart(cell string) (err error) {
	go func() {
		td, _ := GetTidb(cell)
		e := NewEvent(cell, "tidb", "restart")
		defer func() {
			e.Trace(err, fmt.Sprintf("Restart tidb[status=%d]", td.Status))
		}()
		ch := make(chan int, 1)
		if err = Stop(cell, ch); err != nil {
			logs.Error("Delete tidb %s pods on k8s error: %v", cell, err)
			return
		}
		// waiting for all pod deleted
		stoped := <-ch
		if stoped == 0 {
			logs.Error("stop tidb %s timeout", cell)
			return
		}
		if err = Start(cell); err != nil {
			logs.Error("Create tidb %s pods on k8s error: %v", cell, err)
			return
		}
	}()
	return err
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
	if db.Schemas[0].Name != src.Database {
		return fmt.Errorf("both schemas must be the same")
	}
	h, p, err := net.SplitHostPort(db.OuterAddresses[0])
	if err != nil {
		return err
	}
	port, _ := strconv.Atoi(p)
	schema := db.Schemas[0]
	my := &tsql.Migration{
		Src:  src,
		Dest: *tsql.NewMysql(schema.Name, h, port, schema.User, schema.Password),

		ToggleSync: sync,
		NotifyAPI:  notify,
	}
	if err := my.Check(); err != nil {
		return fmt.Errorf(`schema "%s" does not support migration error: %v`, db.Cell, err)
	}
	db.Status.MigrateState = migrating
	if err := db.Update(); err != nil {
		return err
	}
	return db.startMigrateTask(my)
}

// UpdateMigrateStat update tidb migrate stat
func (db *Tidb) UpdateMigrateStat(s, desc string) (err error) {
	var e *Event
	db.Status.MigrateState = s
	if err := db.Update(); err != nil {
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
	s := r.Replace(mysqlMigraeYaml)
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
			err = db.Update()
			return
		}
	}()
	return nil
}

func stopMigrateTask(cell string) error {
	return k8sutil.DeletePodsBy(cell, "migration")
}
