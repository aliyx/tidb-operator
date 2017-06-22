package httputil

import (
	"fmt"
	"testing"
	"time"
)

func TestGet(t *testing.T) {
	b, err := Get("http://10.213.44.128:13390/status", time.Second)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(string(b))
}
