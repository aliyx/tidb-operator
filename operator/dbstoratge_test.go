package operator

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/astaxie/beego"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMain(m *testing.M) {
	beego.AppConfig.Set("k8sAddr", "http://10.213.44.128:10218")
	beego.AppConfig.Set("dockerRegistry", "10.209.224.13:10500/ffan/rds")
	ParseConfig()
	Init()
	os.Exit(m.Run())
}

func TestDb_Save(t *testing.T) {
	db := &Db{
		Owner: &Owner{
			ID:   "6",
			Name: "yangxin45",
		},
		Schema: Schema{
			Name:     "test",
			User:     "test",
			Password: "test",
		},
		Pd: &Pd{
			Spec: Spec{
				Version: "latest",
			},
		},
		Tikv: &Tikv{
			Spec: Spec{
				Version:  "latest",
				Replicas: 3,
			},
			Stores: map[string]*Store{
				"tikv-test-001": &Store{
					Node:  "localhost.localdomain",
					Name:  "tikv-test-001",
					State: StoreOffline,
				},
				"tikv-test-002": &Store{
					Node:  "localhost.localdomain",
					Name:  "tikv-test-002",
					State: StoreOnline,
				},
			},
		},
		Tidb: &Tidb{
			Spec: Spec{
				Version:  "latest",
				Replicas: 1,
			},
		},
	}
	if err := db.Save(); err != nil {
		t.Error(err)
	}
}

func TestGetDb(t *testing.T) {
	db := Db{
		Metadata: metav1.ObjectMeta{
			Name:            "test",
			ResourceVersion: "abc",
		},
	}
	md := Metadata{
		Metadata: metav1.ObjectMeta{
			ResourceVersion: "abcd",
		},
	}
	fmt.Println(reflect.ValueOf(md).FieldByName("Metadata").FieldByName("ResourceVersion").String())
	reflect.ValueOf(&db).Elem().FieldByName("Metadata").FieldByName("ResourceVersion").SetString("abcd")
	fmt.Printf("%+v", db)
}

func TestGetDbs(t *testing.T) {
	dbs, err := GetDbs("admin")
	if err != nil {
		t.Error(err)
	}
	fmt.Println(len(dbs))
	for _, db := range dbs {
		fmt.Printf("%+v", db)
	}
}
