package operator

import (
	"math/rand"
	"sync"
	"time"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/servenv"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
)

const (
	reconcileInterval = 30 * time.Second
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

// Init init all model
func Init(wg *sync.WaitGroup) {
	rand.Seed(time.Now().Unix())
	hook = wg
	k8sutil.Init(beego.AppConfig.String("k8sAddr"))
	onInitHooks.Add(metaInit)
	onInitHooks.Add(dbInit)
	onInitHooks.Add(eventInit)
	onInitHooks.Fire()
}

func recover() {

}
