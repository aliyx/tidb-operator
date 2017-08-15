package k8sutil

import "testing"

func TestDeletePod(t *testing.T) {
	err := DeletePod("tidb-006-xinyang1-mqs2j", 8)
	if err != nil {
		t.Fatal(err)
	}
}
