package mysql

import (
	"fmt"
	"os/exec"
	"testing"
)

func Test_execShell(t *testing.T) {
	fmt.Println(execShell("ls"))
}

func TestMysql_Checker(t *testing.T) {
	m := Mydumper{
		Src: *NewMysql("rds", "10.213.43.158", 10044, "root", "pubsub"),
	}
	err := m.Check()
	if err != nil {
		t.Errorf("%v", err)
	}
}

func TestMysql_execShell(t *testing.T) {
	cmd := exec.Command("sh", "-c", "/home/admin/go/src/github.com/ffan/tidb-k8s/mysql/bin/checker -host 10.213.43.158 -port 10044 -user root -password pubsub rds")
	_, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("%s\n", err)
		// fmt.Printf("%s", stdoutStderr)
	}
	// fmt.Printf("%s\n", stdoutStderr)
}

func Test_creteDir(t *testing.T) {
	dir := "/tmp/test"
	if err := creteDir(dir); err != nil {
		t.Errorf("%v", err)
	}
}

func TestMydumper_Loader(t *testing.T) {
	m := Mydumper{
		Desc:    *NewMysql("cqjtest0", "10.213.129.73", 13779, "cqjtest0", "cqjtest0"),
		DataDir: "/tmp/cqjtest0",
	}
	err := m.Load()
	if err != nil {
		t.Errorf("%v", err)
	}
}

func TestMydumper_Dump(t *testing.T) {
	m := Mydumper{
		Src: *NewMysql("cqjtest0", "10.213.125.70", 13306, "cqjtest0", "cqjtest0"),
	}
	err := m.Dump()
	if err != nil {
		t.Errorf("%v", err)
	}
}

func TestMydumper_Transfer(t *testing.T) {
	m := Mydumper{
		Src:  *NewMysql("cqjtest0", "10.213.125.70", 13306, "cqjtest0", "cqjtest0"),
		Desc: *NewMysql("cqjtest0", "10.213.129.73", 14392, "cqjtest0", "cqjtest0"),
	}
	err := m.Transfer()
	if err != nil {
		t.Errorf("%v", err)
	}
}
