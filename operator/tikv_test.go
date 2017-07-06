package operator

import (
	"fmt"
	"testing"
)

func TestTikv_IsBuried(t *testing.T) {
	db, err := GetDb("006-test")
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("%#v\n", db.Tikv)
	for _, s := range db.Tikv.Stores {
		b, err := db.Tikv.IsBuried(s)
		if err != nil {
			t.Error(err)
		}
		fmt.Printf("store[%s] state: %v\n", s.Name, b)
	}
}

func TestDeleteBuriedTikv(t *testing.T) {
	db, err := GetDb("006-test")
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("%#v\n", db.Tikv)
	if err = DeleteBuriedTikv(db); err != nil {
		t.Error(err)
	}
}
