package mysqlutil

import (
	"os/exec"
	"strings"

	"fmt"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/pkg/servenv"
)

var (
	binDir  = fmt.Sprintf("%s/pkg/mysql/bin/", servenv.Root())
	checker = binDir + "checker -L error -host %s -port %d -user %s -password %s %s"
)

// Migration mysql data to tidb
type Migration struct {
	Src     Mysql
	Dest    Mysql
	Tables  []string
	DataDir string

	ToggleSync bool
	NotifyAPI  string
}

// Check 预先检查 TiDB 是否能支持需要迁移的 table schema
func (m *Migration) Check() error {
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

func execShell(cmd string) ([]byte, error) {
	logs.Info("Command is %s", cmd)
	// splitting head => g++ parts => rest of the command
	parts := strings.Fields(cmd)
	head := parts[0]
	parts = parts[1:len(parts)]
	return exec.Command(head, parts...).CombinedOutput()
}
