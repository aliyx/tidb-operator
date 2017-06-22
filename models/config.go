package models

import (
	"math/rand"
	"os"
	"time"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/pkg/servenv"
)

var (
	forceInitMd   bool
	imageRegistry string
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
	onInitHooks.Add(metaInit)
	onInitHooks.Add(dbInit)
	onInitHooks.Add(eventInit)
	onInitHooks.Fire()
}

// ParseConfig parse all config
func ParseConfig() {
	forceInitMd = beego.AppConfig.DefaultBool("forceInitMd", false)
	imageRegistry = beego.AppConfig.String("dockerRegistry")
	if imageRegistry == "" {
		panic("cannt get images registry address")
	}
}
