package operator

import (
	"math/rand"
	"sync"
	"time"

	"context"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/servenv"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
)

const (
	reconcileInterval = 10 * time.Second
)

var (
	forceInitMd   bool
	imageRegistry string
	onInitHooks   servenv.Hooks
)

// ParseConfig parse all config
func ParseConfig() {
	forceInitMd = beego.AppConfig.DefaultBool("forceInitMd", false)
	imageRegistry = beego.AppConfig.String("dockerRegistry")
	logs.Debug("force init metadata:", forceInitMd)
	logs.Debug("image registrey:", imageRegistry)
}

// Init operator
func Init() {
	rand.Seed(time.Now().Unix())
	k8sutil.Init(beego.AppConfig.String("k8sAddr"))
	onInitHooks.Add(metaInit)
	onInitHooks.Add(dbInit)
	onInitHooks.Add(eventInit)
	onInitHooks.Fire()

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
