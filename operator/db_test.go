package operator

import (
	"testing"
	"time"

	"github.com/astaxie/beego/logs"
	tsql "github.com/ffan/tidb-operator/pkg/util/mysqlutil"
	_ "github.com/go-sql-driver/mysql"
)

func TestDb_startMigrateTask(t *testing.T) {
	db, err := GetDb("006-xinyang1")
	if err != nil {
		t.Fatal(err)
	}
	logs.Debug("db:%v", db)
	if err = stopMigrateTask("006-xinyang1"); err != nil {
		t.Fatal(err)
	}
	my := &tsql.Migration{
		Dest: tsql.Mysql{
			Database: "xinyang1",
			IP:       "10.213.44.128",
			Port:     14446,
			User:     "xinyang1",
			Password: "xinyang1",
		},
		Src: tsql.Mysql{
			Database: "xinyang1",
			IP:       "10.213.124.194",
			Port:     13306,
			User:     "root",
			Password: "EJq4dspojdY3FmVF?TYVBkEMB",
		},
		ToggleSync: true,
		NotifyAPI:  "",
	}
	time.Sleep(6 * time.Second)
	if err = db.startMigrateTask(my); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Second)
}

func TestDb_Migrate(t *testing.T) {
	db, err := GetDb("006-xinyang1")
	if err != nil {
		t.Fatal(err)
	}
	logs.Debug("db:%v", db)
	if err = stopMigrateTask("006-xinyang1"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(6 * time.Second)
	src := tsql.Mysql{
		Database: "xinyang1",
		IP:       "10.213.124.195",
		Port:     13306,
		User:     "root",
		Password: "EJq4dspojdY3FmVF?TYVBkEMB",
	}
	if err = db.Migrate(src, "", true); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Second)
}
