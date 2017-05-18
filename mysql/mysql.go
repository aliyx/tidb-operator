package mysql

import (
	"database/sql"
	"strings"

	"fmt"

	"errors"

	"time"

	"github.com/astaxie/beego/logs"
)

const maxBadConnRetries = 3

var defaultMysqlInitTemplate = `
CREATE DATABASE IF NOT EXISTS {{database}};
DELETE FROM mysql.user WHERE User = '';
CREATE USER '{{user}}'@'%' IDENTIFIED BY '{{password}}';
GRANT ALL ON {{database}}.* TO '{{user}}'@'%' WITH GRANT OPTION;
FLUSH PRIVILEGES;
`

var (
	// tidbDsn tidb data source name
	tidbDsn  = "root@tcp(%s:%d)/mysql?timeout=10s"
	mysqlDsn = "%s:%s@tcp(%s:%d)/%s"
	grants   = "SHOW GRANTS FOR '{{user}}'@'%'"
)

// Mysql 代表一个mysql实例
type Mysql struct {
	Database string `json:"database"`
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
}

// NewMysql crate a mysql
func NewMysql(database, ip string, port int, user, password string) *Mysql {
	return &Mysql{
		Database: database,
		IP:       ip,
		Port:     port,
		User:     user,
		Password: password,
	}
}

func (m Mysql) dsn() string {
	return fmt.Sprintf(tidbDsn, m.IP, m.Port)
}

// Init 初始化mysql
func (m *Mysql) Init() error {
	if err := m.validate(); err != nil {
		return err
	}
	r := strings.NewReplacer("{{database}}", m.Database,
		"{{user}}", m.User,
		"{{password}}", m.Password)
	sqls := strings.Split(r.Replace(defaultMysqlInitTemplate), ";")
	for i, c := range sqls {
		c = strings.Trim(c, "\n")
		c = strings.Trim(c, " ")
		sqls[i] = c
	}
	if err := execMysqlCommand(m.dsn(), sqls...); err != nil {
		return err
	}
	return nil
}

func (m *Mysql) validate() error {
	if m.Database == "" || m.User == "" || m.Password == "" {
		return errors.New("database or user password is nil")
	}
	return nil
}

func execMysqlCommand(dsn string, sqls ...string) error {
	if len(sqls) < 1 {
		return nil
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	for _, c := range sqls {
		if len(c) < 1 {
			return nil
		}
		logs.Debug("dsn: %s sql: %s", dsn, c)
		for i := 0; i < maxBadConnRetries; i++ {
			if _, err = db.Exec(c); err == nil {
				break
			}
			time.Sleep(100 * time.Microsecond)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func havePrivilege(dsn, user string, pri string) (bool, error) {
	all, err := getPrivileges(dsn, user)
	if err != nil {
		return false, err
	}
	for _, p := range all {
		if strings.Contains(p, "GRANT ALL PRIVILEGES ON *.*") {
			return true, nil
		}
	}
	for _, p := range all {
		if strings.Contains(p, pri) {
			return true, nil
		}
	}
	return false, nil
}

// query sql
func getPrivileges(dsn string, user string) ([]string, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	query := strings.Replace(grants, "{{user}}", user, 1)
	logs.Debug("dsn: %s sql: %s", dsn, query)
	rows, err := db.Query(query)
	defer rows.Close()
	ps := []string{}
	for rows.Next() {
		var p string
		if err = rows.Scan(&p); err != nil {
			return nil, err
		}
		ps = append(ps, p)

	}
	err = rows.Err() // get any error encountered during iteration
	if err != nil {
		return nil, err
	}
	return ps, nil
}
