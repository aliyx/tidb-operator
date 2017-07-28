package operator

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
	tsql "github.com/ffan/tidb-operator/pkg/util/mysqlutil"
	"github.com/ghodss/yaml"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	migStarting        = "Migrating"
	migStartMigrateErr = "StartMigrationTaskError"
)

// Migrate migrate the mysql data to the current tidb.
// If 'sync' is true, then starting incremental sync after import all data.
func (db *Db) Migrate(src tsql.Mysql, notify string, sync, include bool, tables []string) error {
	if !db.Status.Available {
		return fmt.Errorf("tidb is not available")
	}
	if len(src.IP) < 1 || src.Port < 1 || len(src.User) < 1 || len(src.Password) < 1 || len(src.Database) < 1 {
		return fmt.Errorf("invalid database: %s", src.Dsn())
	}
	if db.Schema.Name != src.Database {
		return fmt.Errorf("both schemas must be the same")
	}
	if db.Status.MigrateState == "Finished" {
		return fmt.Errorf("the mysql data has been migrated to complete, repeat the migration will lead to data inconsistencies, if you want to migrate, then re-create and then migrate")
	}
	if db.isMigrating() {
		return fmt.Errorf("be migrating")
	}
	sch := db.Schema
	h, p, err := net.SplitHostPort(db.Status.OuterAddresses[0])
	if err != nil {
		return err
	}
	port, _ := strconv.Atoi(p)
	my := &tsql.Migration{
		Src:        src,
		Dest:       *tsql.NewMysql(sch.Name, h, port, sch.User, sch.Password),
		Include:    include,
		Tables:     tables,
		ToggleSync: sync,
		NotifyAPI:  notify,
	}
	logs.Debug("migrator object: %v", my)

	if err := my.Check(); err != nil {
		return fmt.Errorf("schema '%s' does not support migration error: %v", db.GetName(), err)
	}

	db.Operator = "migrate"
	if db.Status.MigrateState != "" {
		db.Status.MigrateState = migStarting
	}
	if err := db.update(); err != nil {
		return err
	}
	return db.startMigrator(my)
}

// SyncMigrateStat update tidb migrate stat
func (db *Db) SyncMigrateStat(stat, reson string) error {
	var (
		err error
		e   *Event
	)
	logs.Info("Current tidb %s migrate status: %s", db.GetName(), stat)
	switch stat {
	case "Finished":
		e = NewEvent(db.GetName(), "migrator", "stop")
		err = db.StopMigrator()
		e.Trace(err, "End the full migrate and delete migrator from k8s")
	case "Syncing":
		e = NewEvent(db.GetName(), "migrator", "sync")
		e.Trace(nil, "Finished load and start incremental syncing mysql data to tidb")
	case "Dumping":
		switch db.Status.MigrateState {
		case "":
			// normal
		case "DumpError":
			// Data has not been imported
			if db.Status.MigrateRetryCount > 10 {
				err = db.StopMigrator()
				e = NewEvent(db.GetName(), "migrator", "stop")
				e.Trace(err, "Delete migrator job, because of more than 10 retries")
			} else {
				db.Status.MigrateRetryCount++
				// retry dump
			}
		default:
			err = db.StopMigrator()
			e = NewEvent(db.GetName(), "migrator", "stop")
			e.Trace(err, "Delete migrator job, becasue of job transfered to another Node, will lead to inconsistent data")
		}

	case "DumpError", "LoadError":
		if db.Status.MigrateRetryCount > 10 {
			err = db.StopMigrator()
			e = NewEvent(db.GetName(), "migrator", "stop")
			e.Trace(err, "Delete migrator job, because of more than 10 retries")
		} else {
			db.Status.MigrateRetryCount++
			// retry dump
		}
	case "Loading":
		// normal
	default:
		return fmt.Errorf("unknow status")
	}
	db.Status.MigrateState = stat
	db.Status.Reason = stat
	if err = db.update(); err != nil {
		return err
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
		e := NewEvent(db.GetName(), "migrator", fmt.Sprintf("operator(%s)", sync))
		defer func() {
			if uerr := db.update(); uerr != nil {
				logs.Error("failed to update db %s error: %v", db.GetName(), uerr)
			}
			e.Trace(err,
				fmt.Sprintf("Migrate mysql(%s) to tidb, include: %v tables: %s", my.Src.Dsn(), my.Include, my.Tables))
		}()

		if _, err = k8sutil.CreateAndWaitJobByJSON(j, waitPodRuningTimeout); err != nil {
			db.Status.MigrateState = migStartMigrateErr
			return
		}
	}()

	return nil
}

// StopMigrator delete migrator
func (db *Db) StopMigrator() error {
	// will delete migrator pod
	return k8sutil.DeleteJob("migrator-" + db.GetName())
}

func (db *Db) isMigrating() bool {
	_, err := k8sutil.GetJob("migrator-" + db.GetName())
	if apierrors.IsNotFound(err) {
		return false
	}
	return true
}
