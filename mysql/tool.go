package mysql

import (
	"os"
	"os/exec"
	"strings"

	"fmt"

	"sync"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/utils"
)

var (
	binDir   = fmt.Sprintf("%s/mysql/bin/", utils.Root())
	checker  = binDir + "checker -L error -host %s -port %d -user %s -password %s %s"
	mydumper = binDir + "mydumper -h %s -P %d -u %s -p %s -t 16 -F 128 -B %s --skip-tz-utc --no-locks -o %s"
	loader   = binDir + "loader -h %s -P %d -u %s -p %s -t 4 -checkpoint=%s -d %s"

	mu sync.Mutex
)

// Mydumper mysql dumper
type Mydumper struct {
	Src     Mysql
	Dest    Mysql
	Tables  []string
	DataDir string
	
	IncrementalSync bool
	NotifyAPI       string
}

// Transfer data from src mysql scheme to tidb scheme
func (m *Mydumper) Transfer() (err error) {
	mu.Lock()
	defer mu.Unlock()
	if err := m.Dump(); err != nil {
		return fmt.Errorf(`dump database "%+v" error: %v`, m.Src, err)
	}
	if err := m.Load(); err != nil {
		return fmt.Errorf(`load data to tidb "%+v" error: %v`, m.Dest, err)
	}
	return nil
}

// Check 预先检查 TiDB 是否能支持需要迁移的 table schema
func (m *Mydumper) Check() error {
	dsn := fmt.Sprintf(mysqlDsn, m.Src.User, m.Src.Password, m.Src.IP, m.Src.Port, m.Src.Database)
	err := execMysqlCommand(dsn, "SELECT 1")
	if err != nil {
		return fmt.Errorf("Ping mysql %s timeout: %v", dsn, err)
	}
	cmd := fmt.Sprintf(checker, m.Src.IP, m.Src.Port, m.Src.User, m.Src.Password, m.Src.Database)
	o, err := execShell(cmd)
	if err != nil {
		return fmt.Errorf("%s", o)
	}
	return nil
}

// Dump mysql数据到/tmp/{database}目录
func (m *Mydumper) Dump() error {
	dir := fmt.Sprintf("/tmp/%s", m.Src.Database)
	m.DataDir = dir
	if err := creteDir(dir); err != nil {
		return err
	}
	cmd := fmt.Sprintf(mydumper, m.Src.IP, m.Src.Port, m.Src.User, m.Src.Password, m.Src.Database, dir)
	o, err := execShell(cmd)
	if err != nil {
		return fmt.Errorf("%v:\n%s", err, o)
	}
	logs.Debug("Dump succ!")
	return nil
}

// Load 数据到tidb
func (m *Mydumper) Load() error {
	checkpoint := fmt.Sprintf("%s/%s", m.DataDir, "loader.checkpoint")
	cmd := fmt.Sprintf(loader, m.Dest.IP, m.Dest.Port, m.Dest.User, m.Dest.Password, checkpoint, m.DataDir)
	o, err := execShell(cmd)
	if err != nil {
		return fmt.Errorf("%v:\n%s", err, o)
	}
	os.RemoveAll(m.DataDir)
	logs.Debug("Load data to tidb succ!")
	return nil
}

func creteDir(dir string) error {
	logs.Debug("Dump data to local temp dir: %s", dir)
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0777); err != nil {
		return err
	}
	return nil
}

func execShell(cmd string) ([]byte, error) {
	logs.Info("Command is %s", cmd)
	// splitting head => g++ parts => rest of the command
	parts := strings.Fields(cmd)
	head := parts[0]
	parts = parts[1:len(parts)]
	return exec.Command(head, parts...).CombinedOutput()
}
