package mysqlutil

import (
	"fmt"
	"os/exec"
	"testing"
)

func Test_execShell(t *testing.T) {
	fmt.Println(execShell("ls"))
}

func TestMigration_Checker(t *testing.T) {
	m := Migration{
		Src:     *NewMysql("xinyang1", "10.213.125.4", 13306, "xinyang1", "xinyang1"),
		Include: false,
		Tables:  []string{"t_error"},
	}
	err := m.Check()
	if err != nil {
		fmt.Printf("check result: %v\n", err)
	}
}

func TestMigration_execShell(t *testing.T) {
	cmd := exec.Command("sh", "-c", "/home/admin/go/src/github.com/ffan/tidb-operator/pkg/mysqlutil/bin/checker -host 10.213.43.158 -port 10044 -user root -password pubsub rds")
	_, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("%s\n", err)
		// fmt.Printf("%s", stdoutStderr)
	}
	// fmt.Printf("%s\n", stdoutStderr)
}
