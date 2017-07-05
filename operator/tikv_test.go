package operator

import "testing"
import "fmt"

func TestTikv_IsBuried(t *testing.T) {
	db, err := GetDb("006-test")
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("%#v", db.Tikv)
	for _, s := range db.Tikv.Stores {
		b, err := db.Tikv.IsBuried(s)
		if err != nil {
			t.Error(err)
		}
		fmt.Printf("store[%s] state: %v", s.Name, b)
	}
}
