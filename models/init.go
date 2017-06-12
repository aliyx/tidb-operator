package models

import (
	"os"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/pkg/servenv"
)

var (
	onInitHooks servenv.Hooks
)

// Init model初始化
func Init() {
	defer func() {
		if err := recover(); err != nil {
			logs.Critical("Init tidb-k8s error: %v", err)
			os.Exit(1)
		}
	}()
	onInitHooks.Add(metaInit)
	onInitHooks.Add(specInit)
	onInitHooks.Add(tidbInit)
	onInitHooks.Add(userInit)
	onInitHooks.Add(eventInit)
	onInitHooks.Fire()
}
