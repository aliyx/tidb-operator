package mysqlutil

import (
	"testing"

	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

func TestMysql_Init(t *testing.T) {
	ms := NewMysql("test1", "10.213.44.128", 13934, "test", "test")
	if err := ms.CreateDatabaseAndGrant(); err != nil {
		t.Errorf("error: %v", err)
	}

}

func Test_havePrivilege(t *testing.T) {
	dsn := fmt.Sprintf(mysqlDsn, "cqjtest0", "cqjtest0", "10.213.129.73", 13053, "cqjtest0")
	ok, err := havePrivilege(dsn, "cqjtest0", "RELOAD")
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("%s privilege: %v\n", "RELOAD", ok)
}
