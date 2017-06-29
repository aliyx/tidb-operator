package garbagecollection

import (
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/operator"
	"github.com/ffan/tidb-operator/pkg/util/prometheusutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	kwatch "k8s.io/apimachinery/pkg/watch"
)

func parse(e watch.Event) (*Event, *metav1.Status) {
	if e.Type == kwatch.Error {
		status := e.Object.(*metav1.Status)
		return nil, status
	}

	db := e.Object.(*operator.Db)
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

func gc(o, n *operator.Db, pv PVProvisioner) (err error) {
	if err = gcPd(o, n); err != nil {
		return err
	}
	if err = gcTikv(o, n, pv); err != nil {
		return err
	}
	if err = gcTidb(o, n); err != nil {
		return err
	}
	return nil
}

func gcPd(o, n *operator.Db) error {
	if n != nil {
		return nil
	}
	if err := prometheusutil.DeleteMetricsByJob(o.Metadata.Name); err != nil {
		return err
	}
	return nil
}

func gcTikv(o, n *operator.Db, pv PVProvisioner) (err error) {
	if o == nil || o.Tikv == nil || len(o.Tikv.Stores) == 0 {
		return nil
	}

	// get deleted tikv

	deleted := make(map[string]*operator.Store)
	if n == nil || n.Tikv == nil || len(n.Tikv.Stores) == 0 {
		deleted = o.Tikv.Stores
	} else {
		newSs := n.Tikv.Stores
		for id, oldS := range o.Tikv.Stores {
			_, ok := newSs[id]
			if !ok {
				deleted[id] = oldS
			}
		}
	}

	// recycle

	logs.Info("recycled tikv: %+v", deleted)

	for id, s := range deleted {
		if s.Node == NodeName {
			logs.Info("recycling tikv: %s", id)
			if err = pv.Recycling(id); err != nil {
				return err
			}
		}
	}

	if n == nil {
		if err = prometheusutil.DeleteMetricsByJob(o.Metadata.Name); err != nil {
			return err
		}
	}
	return nil
}

func gcTidb(o, n *operator.Db) error {
	if n != nil {
		return nil
	}
	if err := prometheusutil.DeleteMetricsByJob(o.Metadata.Name); err != nil {
		return err
	}
	return nil
}
