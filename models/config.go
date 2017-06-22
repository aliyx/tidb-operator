package models

import (
	"math/rand"
	"os"
	"time"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/pkg/k8sutil"
	"github.com/ffan/tidb-k8s/pkg/servenv"
)

var (
	forceInitMd   bool
	imageRegistry string
	k8sAddr       string
	onInitHooks   servenv.Hooks
)

// Init init model
func Init() {
	rand.Seed(time.Now().Unix())

	defer func() {
		if err := recover(); err != nil {
			logs.Critical("Init tidb-k8s error: %v", err)
			os.Exit(1)
		}
	}()
	k8sutil.Init(beego.AppConfig.String("k8sAddr"))
	onInitHooks.Add(metaInit)
	onInitHooks.Add(dbInit)
	onInitHooks.Add(eventInit)
	onInitHooks.Fire()
}

// ParseConfig parse all config
func ParseConfig() {
	k8sAddr = beego.AppConfig.String("k8sAddr")
	forceInitMd = beego.AppConfig.DefaultBool("forceInitMd", false)
	imageRegistry = beego.AppConfig.String("dockerRegistry")
	if imageRegistry == "" {
		panic("cannt get images registry address")
	}
}
