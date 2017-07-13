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

func TestTikv_checkStoresStatus(t *testing.T) {
	db, err := GetDb("006-xinyang1")
	if err != nil {
		t.Fatal(err)
	}
	c, err := db.Tikv.checkStoresStatus()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("count of changed stores is %d\n", c)
}
