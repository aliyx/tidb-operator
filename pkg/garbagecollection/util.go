package garbagecollection

import (
	"time"

	"github.com/ffan/tidb-k8s/models"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	kwatch "k8s.io/apimachinery/pkg/watch"
)

func parse(e watch.Event) (*Event, *metav1.Status) {
	if e.Type == kwatch.Error {
		status := e.Object.(*metav1.Status)
		return nil, status
	}

	db := e.Object.(*models.Db)
	ev := &Event{
		Type:   e.Type,
		Object: db,
	}
	return ev, nil
}

// panicTimer panics when it reaches the given duration.
type panicTimer struct {
	d   time.Duration
	msg string
	t   *time.Timer
}

func newPanicTimer(d time.Duration, msg string) *panicTimer {
	return &panicTimer{
		d:   d,
		msg: msg,
	}
}

func (pt *panicTimer) start() {
	pt.t = time.AfterFunc(pt.d, func() {
		panic(pt.msg)
	})
}

// stop stops the timer and resets the elapsed duration.
func (pt *panicTimer) stop() {
	if pt.t != nil {
		pt.t.Stop()
		pt.t = nil
	}
}
