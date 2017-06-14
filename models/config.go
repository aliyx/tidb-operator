package models

import (
	"time"

	"github.com/astaxie/beego"
)

const (
	// Path components
	tidbRoot       = "/tk/"
	tidbNamespace  = tidbRoot + "tidbs"
	metaNamespace  = tidbRoot + "metadata"
	eventNamespace = tidbRoot + "events"

	// pdReqTimeout access the request timeout for the pd api service
	pdReqTimeout = 3 * time.Second
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
