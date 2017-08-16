package test

import (
	"testing"

	"fmt"

	"time"

	"encoding/json"

	"github.com/ffan/tidb-operator/operator/controllers"
	"github.com/ffan/tidb-operator/pkg/util/httputil"
	"github.com/ffan/tidb-operator/pkg/util/mysqlutil"
)

const (
	host = "http://127.0.0.1:12808"

	createDBAPI = "%s/tidb/api/v1/tidbs/"
	deleteDBAPI = "%s/tidb/api/v1/tidbs/%s"
	migrateAPI  = "%s/tidb/api/v1/tidbs/%s/migrate"
	eventsAPI   = "%s/tidb/api/v1/tidbs/%s/events"
	limitAPI    = "%s/tidb/api/v1/tidbs/%s/limit"
	metadataAPI = "%s/tidb/api/v1/metadata"
)

func Test_GetMetadata(t *testing.T) {
	resp, err := httputil.Get(fmt.Sprintf(metadataAPI, host), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%s", resp)
}

func Test_Limit(t *testing.T) {
	body := `{"kvReplicas":4,"dbReplicas":5}`
	resp, err := httputil.Post(fmt.Sprintf(limitAPI, host, "6"), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("need approval: " + resp)
}

func Test_CreateDB(t *testing.T) {
	body := `{
		"pd":{"version":"rc4"},"tikv":{"replicas":3,"version":"rc4"},
		"tidb":{"replicas":2,"version":"rc4"},
		"owner":{"userId":"1","userName":"test","desc":""},
		"schema":{"name":"test","user":"test","password":"test"},
		"status":{"phase":-1}}`
	resp, err := httputil.Post(fmt.Sprintf(createDBAPI, host), []byte(body))
	if err == httputil.ErrAlreadyExists {
		fmt.Println("already exist")
		return
	} else if err != nil {
		t.Fatal(err)
	}
	fmt.Println("id:" + resp)
}

func Test_GetDB(t *testing.T) {
	resp, err := httputil.Get(fmt.Sprintf(deleteDBAPI, host, "001-test"), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%s", resp)
}

func Test_AuditPass(t *testing.T) {
	body := `[{"op":"replace","path":"/operator","value":"audit"},
	{"op":"replace","path":"/status/phase","value":0}]`
	err := httputil.Patch(fmt.Sprintf(deleteDBAPI, host, "001-test"), []byte(body), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_AuditRefuse(t *testing.T) {
	body := `[{"op":"replace","path":"/operator","value":"audit"},
	{"op":"replace","path":"/status/phase","value":-2},
	{"op":"replace","path":"/owner/reason","value":"refuse"}]`
	err := httputil.Patch(fmt.Sprintf(deleteDBAPI, host, "001-test"), []byte(body), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_Start(t *testing.T) {
	body := `[{"op":"replace","path":"/operator","value":"start"}]`
	err := httputil.Patch(fmt.Sprintf(deleteDBAPI, host, "001-test"), []byte(body), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_Stop(t *testing.T) {
	body := `[{"op":"replace","path":"/operator","value":"stop"}]`
	err := httputil.Patch(fmt.Sprintf(deleteDBAPI, host, "001-test"), []byte(body), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_Restart(t *testing.T) {
	body := `[{"op":"replace","path":"/operator","value":"restart"}]`
	err := httputil.Patch(fmt.Sprintf(deleteDBAPI, host, "001-test"), []byte(body), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_Upgrade(t *testing.T) {
	body := `[{"op":"replace","path":"/operator","value":"upgrade"},
	{"op":"replace","path":"/pd/version","value":"latest"},
	{"op":"replace","path":"/tikv/version","value":"latest"},
	{"op":"replace","path":"/tidb/version","value":"latest"}]`
	err := httputil.Patch(fmt.Sprintf(deleteDBAPI, host, "001-test"), []byte(body), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_Scale(t *testing.T) {
	body := `[{"op":"replace","path":"/operator","value":"scale"},
	{"op":"replace","path":"/tidb/replicas","value":2},
	{"op":"replace","path":"/tikv/replicas","value":4}]`
	err := httputil.Patch(fmt.Sprintf(deleteDBAPI, host, "001-test"), []byte(body), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_DeleteDB(t *testing.T) {
	err := httputil.Delete(fmt.Sprintf(deleteDBAPI, host, "001-test"), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

/*
CREATE TABLE t1 (id INT, age INT, PRIMARY KEY(id)) ENGINE=InnoDB;
CREATE TABLE t2 (id INT, name VARCHAR(256), PRIMARY KEY(id)) ENGINE=InnoDB;
CREATE TABLE t_error (
  c timestamp(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
INSERT INTO t1 VALUES (1, 1), (2, 2), (3, 3);
INSERT INTO t2 VALUES (1, "a"), (2, "b"), (3, "c");
*/
func Test_Migrate(t *testing.T) {
	m := controllers.Migrator{
		Mysql: mysqlutil.Mysql{
			Database: "test",
			IP:       "10.213.125.107",
			Port:     13306,
			User:     "test",
			Password: "test",
		},
		Include: true,
		Tables:  []string{"t1", "t2"},
		Sync:    false,
		Notify:  false,
	}
	body, _ := json.Marshal(m)
	fmt.Printf("%s", body)
	_, err := httputil.Post(fmt.Sprintf(migrateAPI, host, "001-test"), body)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_GetEvents(t *testing.T) {
	b, err := httputil.Get(fmt.Sprintf(eventsAPI, host, "001-test"), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%s\n", b)
}
