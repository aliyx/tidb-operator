package models

import (
	"fmt"

	"time"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

const (
	// Path components
	tidbRoot       = "/tk/tidb"
	tidbNamespace  = "tidbs"
	metaNamespace  = "metadata"
	userNamespace  = "users"
	eventNamespace = "events"

	// k8sReqTimeout 访问k8s api的tiemeout
	k8sReqTimeout = 3 * time.Second
	// pdReqTimeout 访问pd api服务的请求imeout
	pdReqTimeout = 3 * time.Second
	// startTidbTimeout 启动tidb三个子模块pd/tikv/tidb的timeout
	startTidbTimeout = 60 * time.Second
	// storageTimeout 数据存储的timeout
	storageTimeout = 3 * time.Second
)

var (
	k8sAddr       string
	etcdAddress   string
	forceInitMd   bool
	pdScaleFactor float64
)

func configInit() {
	k8s := beego.AppConfig.String("k8sAddr")
	if k8s == "" {
		panic(fmt.Errorf("No set kubernetes master addr"))
	}
	k8sAddr = k8s

	addr := beego.AppConfig.String("etcdAddress")
	if addr == "" || len(addr) == 0 {
		logs.Critical("The etcd address can not be empty")
	}
	etcdAddress = addr

	md, err := beego.AppConfig.Bool("forceInitMd")
	if err != nil {
		logs.Warn("No set forceInitMd")
	} else {
		forceInitMd = md
	}

	sf, err := beego.AppConfig.Float("pdScaleFactor")
	if err != nil {
		logs.Warn("No set pdScaleFactor")
	} else {
		pdScaleFactor = sf
	}
}
