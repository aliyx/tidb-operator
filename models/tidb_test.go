package models

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestTidb_check(t *testing.T) {
	db := &Tidb{
		Cell: "",
		Owner: &Owner{
			ID:   "25",
			Name: "yangxin45",
		},
		Schemas: []Schema{
			Schema{Name: "test", User: "test", Password: "test"},
		},
		Spec: Spec{
			CPU:      200,
			Mem:      256,
			Replicas: 1,
			Version:  "latest",
		},
		Pd: &Pd{
			Spec: Spec{
				CPU:      200,
				Mem:      256,
				Replicas: 3,
				Version:  "latest",
			},
		},
		Tikv: &Tikv{
			Spec: Spec{
				CPU:      200,
				Mem:      256,
				Replicas: 3,
				Version:  "latest",
			},
		},
	}
	bs, _ := json.Marshal(db)
	fmt.Printf("%s\n", bs)
}
