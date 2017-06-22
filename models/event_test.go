package models

import (
	"testing"
)

func TestEvent_Trace(t *testing.T) {
	e := NewEvent("test", "test", "test")
	e.Trace(nil, "test")
}
