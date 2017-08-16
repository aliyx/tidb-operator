package pdutil

import (
	"testing"
)

func TestPdStoreDelete(t *testing.T) {
	err := PdStoreDelete("10.213.44.128:13120", 10)
	if err != nil {
		t.Error(err)
	}
}

func TestPdStoreIDGet(t *testing.T) {
	string, err := PdStoreIDGet("10.213.44.128:13120", 4)
	if err != nil {
		t.Error(err)
	}
	println(string)
}

func TestPdMemberDelete(t *testing.T) {
	err := PdMemberDelete("10.213.44.128:14854", "pd-001-test-003")
	if err != nil {
		t.Error(err)
	}
}
