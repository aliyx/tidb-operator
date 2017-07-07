package test

import (
	"testing"

	"fmt"

	"time"

	"github.com/ffan/tidb-operator/pkg/util/httputil"
)

const (
	host = "http://127.0.0.1:12808"

	createDBAPI = "%s/tidb/api/v1/tidbs/"
	deleteDBAPI = "%s/tidb/api/v1/tidbs/%s"
	limitAPI    = "%s/tidb/api/v1/tidbs/%s/limit"
)

func Test_Limit(t *testing.T) {
	body := `{"kvr":4,"dbr":5}`
	resp, err := httputil.Post(fmt.Sprintf(limitAPI, host, "6"), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(resp)
}

func Test_CreateDB(t *testing.T) {
	body := `{
		"pd":{"version":"rc3"},"tikv":{"replicas":3,"version":"rc3"},
		"tidb":{"replicas":2,"version":"rc3"},
		"owner":{"userId":"6","userName":"yangxin45","desc":""},
		"schema":{"name":"xinyang1","user":"xinyang1","password":"xinyang1"},
		"status":{"phase":-1}}`
	resp, err := httputil.Post(fmt.Sprintf(createDBAPI, host), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(resp)
}

func Test_GetDB(t *testing.T) {
	resp, err := httputil.Get(fmt.Sprintf(deleteDBAPI, host, "006-xinyang1"), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%s", resp)
}

func Test_AuditPass(t *testing.T) {
	body := `[{"op":"replace","path":"/operator","value":"audit"},
	{"op":"replace","path":"/status/phase","value":0}]`
	err := httputil.Patch(fmt.Sprintf(deleteDBAPI, host, "006-xinyang1"), []byte(body), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_AuditRefuse(t *testing.T) {
	body := `[{"op":"replace","path":"/operator","value":"audit"},
	{"op":"replace","path":"/status/phase","value":-2},
	{"op":"replace","path":"/owner/reason","value":"refuse"}]`
	err := httputil.Patch(fmt.Sprintf(deleteDBAPI, host, "006-xinyang1"), []byte(body), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_Start(t *testing.T) {
	body := `[{"op":"replace","path":"/operator","value":"start"}]`
	err := httputil.Patch(fmt.Sprintf(deleteDBAPI, host, "006-xinyang1"), []byte(body), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_Stop(t *testing.T) {
	body := `[{"op":"replace","path":"/operator","value":"stop"}]`
	err := httputil.Patch(fmt.Sprintf(deleteDBAPI, host, "006-xinyang1"), []byte(body), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_Restart(t *testing.T) {
	body := `[{"op":"replace","path":"/operator","value":"restart"}]`
	err := httputil.Patch(fmt.Sprintf(deleteDBAPI, host, "006-xinyang1"), []byte(body), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_Upgrade(t *testing.T) {
	body := `[{"op":"replace","path":"/operator","value":"upgrade"},
	{"op":"replace","path":"/pd/version","value":"latest"},
	{"op":"replace","path":"/tikv/version","value":"latest"},
	{"op":"replace","path":"/tidb/version","value":"latest"}]`
	err := httputil.Patch(fmt.Sprintf(deleteDBAPI, host, "006-xinyang1"), []byte(body), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_Scale(t *testing.T) {
	body := `[{"op":"replace","path":"/operator","value":"scale"},
	{"op":"replace","path":"/tidb/replicas","value":2},
	{"op":"replace","path":"/tikv/replicas","value":4}]`
	err := httputil.Patch(fmt.Sprintf(deleteDBAPI, host, "006-xinyang1"), []byte(body), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_DeleteDB(t *testing.T) {
	err := httputil.Delete(fmt.Sprintf(deleteDBAPI, host, "006-xinyang1"), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
}
