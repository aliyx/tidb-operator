package models

import (
	"fmt"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	k8sAddr = "http://10.213.44.128:10218"
)

// func TestMain(m *testing.M) {
// 	k8sutil.Init(k8sAddr)
// 	Init()
// 	os.Exit(m.Run())
// }

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
		},
		Tidb: &Tidb{
			Spec: Spec{
				Version:  "latest",
				Replicas: 1,
			},
		},
	}
	// if err := db.Save(); err != nil {
	// 	t.Error(err)
	// }
	fmt.Printf("%+v", *db)
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
	type args struct {
		userID string
	}
	tests := []struct {
		name    string
		args    args
		want    []*Db
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetDbs(tt.args.userID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDbs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetDbs() = %v, want %v", got, tt.want)
			}
		})
	}
}
