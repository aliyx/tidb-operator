package operator

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	batchv1 "k8s.io/api/batch/v1"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
	tsql "github.com/ffan/tidb-operator/pkg/util/mysqlutil"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

const migCmd = `
migrator \
--database {{db}} \
--src-host {{sh}} \
--src-port {{sP}} \
--src-user {{su}} \
--src-password {{sp}} \
--dest-host {{dh}} \
--dest-port {{dP}} \
--dest-user {{du}} \
--dest-password {{dp}} \
--operator {{op}} \
--tables "{{tables}}" \
--notice "{{api}}"
`

func (db *Db) startMigrator(my *tsql.Migration) (err error) {
	sync := "load"
	if my.ToggleSync {
		sync = "sync"
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "migrator-" + db.GetName(),
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "migrator-" + db.GetName(),
					Labels: db.getLabels("migrator"),
				},
				Spec: v1.PodSpec{
					RestartPolicy:                 v1.RestartPolicyOnFailure,
					TerminationGracePeriodSeconds: getTerminationGracePeriodSeconds(),
					Containers: []v1.Container{
						v1.Container{
							Name:  "migrator",
							Image: imageRegistry + "migrator:latest",
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("200m"),
									v1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
							Env: []v1.EnvVar{
								v1.EnvVar{
									Name:  "TZ",
									Value: "Asia/Shanghai",
								},
							},
							Command: []string{
								"bash", "-c", fmt.Sprintf("migrator"+
									" --database %s"+
									" --src-host %s --src-port %d --src-user %s --src-password %s"+
									" --dest-host %s --dest-port %d --dest-user %s --dest-password %s"+
									" --operator %s"+
									" --tables %q"+
									" --notice %q", my.Src.Database, my.Src.IP, my.Src.Port, my.Src.User, my.Src.Password, my.Dest.IP, my.Dest.Port, my.Dest.User, my.Dest.Password, sync, strings.Join(my.Tables, ","), my.NotifyAPI),
							},
						},
					},
				},
			},
		},
	}

	go func() {
		e := db.Event(eventMigrator, fmt.Sprintf("operator(%s)", sync))
		defer func() {
			e.Trace(err,
				fmt.Sprintf("Migrate mysql(%s) to tidb, include: %v tables: %s", my.Src.Dsn(), my.Include, my.Tables))
		}()

		if _, err = k8sutil.CreateAndWaitJob(job, waitPodRuningTimeout); err != nil {
			if uerr := db.patch(func(newDb *Db) {
				newDb.Status.MigrateState = migStartMigrateErr
			}); uerr != nil {
				db.Event(eventMigrator, "update").Trace(uerr, "Failed to update db")
			}
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
