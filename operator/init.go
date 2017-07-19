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
	reconcileInterval = 60 * time.Second
)

var (
	forceInitMd   bool
	imageRegistry string
	onInitHooks   servenv.Hooks
	hook          *sync.WaitGroup
)

// ParseConfig parse all config
func ParseConfig() {
	forceInitMd = beego.AppConfig.DefaultBool("forceInitMd", false)
	imageRegistry = beego.AppConfig.String("dockerRegistry")
	logs.Debug("force init metadata: ", forceInitMd)
	logs.Debug("image registrey: ", imageRegistry)
}

func Init() {
	rand.Seed(time.Now().Unix())
	k8sutil.Init(beego.AppConfig.String("k8sAddr"))
	onInitHooks.Add(metaInit)
	onInitHooks.Add(dbInit)
	onInitHooks.Add(eventInit)
	onInitHooks.Fire()

}

func Run(ctx context.Context, wg *sync.WaitGroup) (err error) {
	hook = wg

	if err = recover(); err != nil {
		return err
	}

	reconcile(ctx)
	return nil
}

func recover() error {
	dbs, err := GetDbs("admin")
	if err != nil {
		return err
	}
	for i := range dbs {
		db := &dbs[i]
		db.AfterPropertiesSet()
		// recover scaling to normal
		if db.Status.ScaleState&scaling > 0 {
			db.Status.ScaleState ^= scaling
			if err = db.update(); err != nil {
				return err
			}
			logs.Warn("recover db %s", db.GetName())
		}
	}
	return nil
}

func reconcile(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				logs.Warn("reconcile cancled")
				return
			default:
				time.Sleep(reconcileInterval)
			}
			logs.Info("reconcile all tidb clusters")

			dbs, err := GetDbs("admin")
			if err != nil {
				logs.Error("failed to get all tidb clusters")
			}
			for i := range dbs {
				db := &dbs[i]
				db.AfterPropertiesSet()
				if err = db.Scale(db.Tikv.Replicas, db.Tidb.Replicas); err != nil {
					switch err {
					case ErrScaling, ErrUnavailable:
						logs.Warn("%s %v", db.GetName(), err)
					default:
						logs.Error("failed to reconcile db %s: %v", db.GetName(), err)
					}
				}
			}
		}
	}()
}
