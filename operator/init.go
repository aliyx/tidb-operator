package operator

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"context"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/spec"
	"github.com/ffan/tidb-operator/pkg/storage"
)

const (
	reconcileInterval = 15 * time.Second

	defaultImageVersion = "latest"
)

var (
	// ForceInitMd overwrite metadata if true, otherwise not
	ForceInitMd bool
	// ImageRegistry private docker image registry
	ImageRegistry string
	// HostPath as tikv root directory on the host node’s filesystem
	HostPath string
	// Mount as tikv subpath prefix
	Mount string

	dbS  *storage.Storage
	evtS *storage.Storage
)

// Init operator
func Init() {
	rand.Seed(time.Now().Unix())
	metaInit()

	s, err := storage.NewStorage(getNamespace(), spec.CRDKindTidb)
	if err != nil {
		panic(fmt.Errorf("Create storage db error: %v", err))
	}
	dbS = s

	s, err = storage.NewStorage(getNamespace(), spec.CRDKindEvent)
	if err != nil {
		panic(fmt.Errorf("Create storage event error: %v", err))
	}
	evtS = s
}

// Run operator check
func Run(ctx context.Context) (err error) {
	if err = undo(); err != nil {
		return
	}

	go reconcile(ctx)
	return
}

func undo() error {
	dbs, err := GetDbs("admin")
	if err != nil {
		return err
	}
	for i := range dbs {
		db := &dbs[i]
		db.AfterPropertiesSet()

		// init locker
		mu.Lock()
		lockers[db.GetName()] = new(sync.Mutex)
		mu.Unlock()

		changed := false
		// recover scaling to normal
		if db.Status.ScaleState&scaling > 0 {
			db.Status.ScaleState ^= scaling
			changed = true
		}
		// recover upgrade to normal
		if db.Status.UpgradeState == upgrading {
			db.Status.UpgradeState = ""
			changed = true
		}
		if changed {
			if err = db.update(); err != nil {
				return err
			}
			logs.Warn("recover db %s", db.GetName())
		}

		// recover if the installation process is interrupted
		if db.Status.Phase > PhaseUndefined && db.Status.Phase < PhaseTidbInited {
			switch db.Operator {
			case "start", "restart":
				logs.Warn("recover db %q to reinstall", db.GetName())
				go db.Reinstall()
			}
		}
		if db.Status.Phase > PhaseTidbInited {
			switch db.Operator {
			case "restart":
				logs.Warn("recover db %q to reinstall", db.GetName())
				go db.Reinstall()
			case "stop":
				logs.Warn("recover db %q to uninstall", db.GetName())
				go db.Uninstall(true)
			}
		}
	}
	return nil
}

func reconcile(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			logs.Warn("reconcile cancled")
			return
		default:
			time.Sleep(reconcileInterval)
		}
		logs.Debug("start reconciling all tidb clusters")

		dbs, err := GetDbs("admin")
		if err != nil {
			logs.Error("failed to get all tidb clusters")
			continue
		}
		for i := range dbs {
			db := &dbs[i]
			db.AfterPropertiesSet()
			// no pass
			if db.Status.Phase <= PhaseUndefined {
				continue
			}
			if db.Doing() {
				logs.Info("db %q is busy", db.GetName())
				continue
			}
			go func() {
				err := db.Reconcile()
				if err != nil {
					switch err {
					case ErrUnavailable:
						if db.Status.Phase > PhaseUndefined {
							logs.Critical("db %q is not available", db.GetName())
						}
					default:
						logs.Error("failed to reconcile db %q: %v", db.GetName(), err)
					}
				}
			}()
		}
	}
}
