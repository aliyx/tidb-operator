package controllers

import (
	"testing"

	"fmt"

	"github.com/ffan/tidb-operator/operator"
)

func Test_patch(t *testing.T) {
	r := `
[
  { "op": "replace", "path": "/tikv/replicas", "value": 4 },
  { "op": "replace", "path": "/tidb/replicas", "value": 1 }
]`
	type args struct {
		b []byte
		v interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "replace",
			args: args{
				b: []byte(r),
				v: &operator.Db{
					Tikv: &operator.Tikv{},
					Tidb: &operator.Tidb{},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := patch(tt.args.b, tt.args.v); (err != nil) != tt.wantErr {
				t.Errorf("patch() error = %v, wantErr %v", err, tt.wantErr)
			} else {
				db := tt.args.v.(*operator.Db)
				fmt.Printf("replicas:%d\n", db.Tikv.Replicas)
			}
		})
	}
}
