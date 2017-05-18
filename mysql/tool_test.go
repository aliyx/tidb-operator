package mysql

import (
	"fmt"
	"testing"
)

func Test_execShell(t *testing.T) {
	fmt.Println(execShell("ls"))
}

func TestMysql_Checker(t *testing.T) {
	m := Mydumper{
		Src: *NewMysql("cqjtest0", "10.213.125.70", 14392, "cqjtest0", "cqjtest0"),
	}
	err := m.Check()
	if err != nil {
		t.Errorf("%v", err)
	}
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
