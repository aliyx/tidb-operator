package models

import (
	"testing"
)

func TestEvent_Trace(t *testing.T) {
	e := NewEvent("test", "test", "test1")
	e.Trace(nil, "test")
	e = NewEvent("test", "test", "test2")
	e.Trace(nil, "test")
}
