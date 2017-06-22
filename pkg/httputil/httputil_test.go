package httputil

import (
	"fmt"
	"testing"
	"time"
)

func TestGet(t *testing.T) {
	url := fmt.Sprintf("http://%s/pd/api/v1/stores", "10.213.44.128:13389")
	b, err := Get(url, time.Second)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(string(b))
}
