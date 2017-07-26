package mysqlutil

import (
	"database/sql"
	"os/exec"
	"strings"

	"fmt"

	"errors"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/servenv"
)

var (
	binDir  = fmt.Sprintf("%s/pkg/util/mysqlutil/bin/", servenv.Root())
	checker = binDir + "checker -L error -host %s -port %d -user %s -password %s %s"

	sqlGetTablesName = "SELECT table_name FROM information_schema.tables where table_schema='%s'"

	errNoReplicationClientPri = errors.New("No replication client privilege or no super user")
)

// Migration mysql data to tidb
type Migration struct {
	Src     Mysql
	Dest    Mysql
	Tables  []string
	Include bool
	DataDir string

	ToggleSync bool
	NotifyAPI  string
}

// Check 预先检查 TiDB 是否能支持需要迁移的 table schema
func (m *Migration) Check() error {
	dsn := m.Src.Dsn()
	err := execMysqlCommand(dsn, "SELECT 1")
	if err != nil {
		return fmt.Errorf("ping mysql %s timeout: %v", dsn, err)
	}
	cmd := fmt.Sprintf(checker, m.Src.IP, m.Src.Port, m.Src.User, m.Src.Password, m.Src.Database)
	if len(m.Tables) > 0 {
		tables := []string{}
		if m.Include {
			for _, t := range m.Tables {
				tables = append(tables, t)
			}
		} else {
			all, err := m.getAllTablesName()
			if err != nil {
				return err
			}
			for _, t := range all {
				ck := true
				for _, f := range m.Tables {
					if t == f {
						ck = false
						break
					}
				}
				if ck {
					tables = append(tables, t)
				}
			}
		}
		cmd += (" " + strings.TrimRight(strings.TrimLeft(fmt.Sprintf("%s", tables), "["), "]"))
	}
	o, err := execShell(cmd)
	if err != nil {
		return fmt.Errorf("%s", o)
	}
	if m.ToggleSync {
		have, err := havePrivilege(dsn, m.Src.User, "REPLICATION CLIENT")
		if err != nil {
			return err
		}
		if !have {
			return errNoReplicationClientPri
		}
	}
	return nil
}

func (m *Migration) getAllTablesName() ([]string, error) {
	dsn := m.Src.Dsn()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := fmt.Sprintf(sqlGetTablesName, m.Src.Database)
	logs.Debug("dsn: %s sql: %s", dsn, query)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tablesName := []string{}
	for rows.Next() {
		var p string
		if err = rows.Scan(&p); err != nil {
			return nil, err
		}
		tablesName = append(tablesName, p)
	}
	err = rows.Err() // get any error encountered during iteration
	if err != nil {
		return nil, err
	}
	return tablesName, nil
}

func execShell(cmd string) ([]byte, error) {
	logs.Info("Command is %s", cmd)
	parts := strings.Fields(cmd)
	head := parts[0]
	parts = parts[1:len(parts)]
	return exec.Command(head, parts...).CombinedOutput()
}
