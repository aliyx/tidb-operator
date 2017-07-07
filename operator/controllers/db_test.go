package controllers

import (
	"testing"

	"github.com/ffan/tidb-operator/operator"
)

var patchstr = `
[
  { "op": "replace", "path": "/tikv/replicas", "value": 4 },
  { "op": "replace", "path": "/tidb/replicas", "value": 1 }
]
`

func TestTidbController_Patch(t *testing.T) {
	db := operator.Db{
		Pd: &operator.Pd{
			Spec: operator.Spec{
				Version: "latest",
			},
		},
	}
	if err := patch([]byte(patchstr), &db); err != nil {
		t.Error(err)
	}
	println(db.Pd.Version)
}
