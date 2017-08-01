package operator

import "testing"

func TestDb_StopMigrator(t *testing.T) {
	db := NewDb()
	db.Metadata.Name = "006-xinyang1"
	err := db.StopMigrator()
	if err != nil {
		t.Fatal(err)
	}
}
