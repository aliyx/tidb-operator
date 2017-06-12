package models

import (
	"time"

	"github.com/astaxie/beego"
)

const (
	// Path components
	tidbRoot       = "/tk/tidb"
	tidbNamespace  = "tidbs"
	metaNamespace  = "metadata"
	userNamespace  = "users"
	eventNamespace = "events"

	// pdReqTimeout access the request timeout for the pd api service
	pdReqTimeout = 3 * time.Second
	// startTidbTimeout start tidb three sub-module pd / tikv / tidb timeout
	startTidbTimeout = 60 * time.Second
	// storageTimeout data storage timeout
	storageTimeout = 3 * time.Second
)

var (
	etcdAddress   string // storage
	forceInitMd   bool
	imageRegistry string

	defaultImageRegistry = "10.209.224.13:10500/ffan/rds"
)

func init() {
	etcdAddress = beego.AppConfig.String("etcdAddress")
	forceInitMd = beego.AppConfig.DefaultBool("forceInitMd", false)
	imageRegistry = beego.AppConfig.DefaultString("dockerRegistry", defaultImageRegistry)
}
