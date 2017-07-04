package operator

import (
	"math/rand"
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
)

// ParseConfig parse all config
func ParseConfig() {
	forceInitMd = beego.AppConfig.DefaultBool("forceInitMd", false)
	imageRegistry = beego.AppConfig.String("dockerRegistry")
	logs.Debug("force init metadata: ", forceInitMd)
	logs.Debug("image registrey: ", imageRegistry)
}

// Init init all model
func Init() {
	rand.Seed(time.Now().Unix())

	k8sutil.Init(beego.AppConfig.String("k8sAddr"))
	onInitHooks.Add(metaInit)
	onInitHooks.Add(dbInit)
	onInitHooks.Add(eventInit)
	onInitHooks.Fire()
}