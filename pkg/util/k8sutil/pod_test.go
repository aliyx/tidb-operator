package k8sutil

import "testing"

func TestDeletePod(t *testing.T) {
	err := DeletePod("tidb-006-xinyang1-528wn", 8)
	if err != nil {
		t.Fatal(err)
	}
}
