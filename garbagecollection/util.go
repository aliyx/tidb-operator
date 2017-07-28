package garbagecollection

import (
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/operator"
	"github.com/ffan/tidb-operator/pkg/util/prometheusutil"

	"reflect"

	"fmt"

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
	pd := o.Pd
	if pd == nil {
		return nil
	}
	for _, mem := range pd.Members {
		if err := prometheusutil.DeleteMetrics(o.GetName(), mem.Name); err != nil {
			return err
		}
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
	if len(deleted) > 0 {
		logs.Info("tikv %v to be recycled", reflect.ValueOf(deleted).MapKeys())
	}

	for _, s := range deleted {
		if err = pv.Recycling(s); err != nil {
			return err
		}
		// job is tikv_{id}
		if err = prometheusutil.DeleteMetrics(fmt.Sprintf("tikv_%d", s.ID), s.Name); err != nil {
			return err
		}
	}

	// delete all mitric by job
	if n == nil {
		if err = prometheusutil.DeleteMetricsByJob(o.GetName()); err != nil {
			return err
		}
	}
	return nil
}

func gcTidb(o, n *operator.Db) error {
	// no initialization done
	if o.Tidb == nil || len(o.Tidb.Members) == 0 {
		return nil
	}

	deleted := []string{}
	if n == nil || n.Tidb == nil || len(n.Tidb.Members) == 0 {
		for _, mb := range o.Tidb.Members {
			deleted = append(deleted, mb.Name)
		}
	} else {
		for _, oM := range o.Tidb.Members {
			have := false
			for _, nM := range n.Tidb.Members {
				if oM.Name == nM.Name {
					have = true
					break
				}
			}
			if !have {
				deleted = append(deleted, oM.Name)
			}
		}
	}
	for _, name := range deleted {
		// all tidb job is 'tidb'

		if err := prometheusutil.DeleteMetrics("tidb", name); err != nil {
			return err
		}
		// with port metrics
		if err := prometheusutil.DeleteMetrics("tidb", fmt.Sprintf("%s_%d", name, 4000)); err != nil {
			return err
		}
	}
	if n == nil {
		if err := prometheusutil.DeleteMetricsByJob(o.GetName()); err != nil {
			return err
		}
	}
	return nil
}
