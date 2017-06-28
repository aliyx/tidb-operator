package garbagecollection

import (
	"os"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/models"
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

func gc(o, n *models.Db, pv PVProvisioner) (err error) {
	// if err = gcPd(o, n); err != nil {
	// 	return err
	// }
	if err = gcTikv(o, n, pv); err != nil {
		return err
	}
	if err = gcTidb(o, n); err != nil {
		return err
	}
	return nil
}

func gcPd(o, n *models.Db) error {
	if n != nil {
		return nil
	}
	if err := prometheusutil.DeleteMetricsByJob(o.Metadata.Name); err != nil {
		return err
	}
	return nil
}

func gcTikv(o, n *models.Db, pv PVProvisioner) error {
	if o == nil || o.Tikv == nil || len(o.Tikv.Stores) == 0 {
		return nil
	}

	// get deleted tikv

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	deleted := make(map[string]*models.Store)
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

	for id, s := range deleted {
		logs.Debug("%s %s", hostname, id)
		if s.Node == hostname {
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

func gcTidb(o, n *models.Db) error {
	if n != nil {
		return nil
	}
	if err := prometheusutil.DeleteMetricsByJob(o.Metadata.Name); err != nil {
		return err
	}
	return nil
}
