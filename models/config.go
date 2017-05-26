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

	// k8sAPITimeout access k8s api tiemeout
	k8sAPITimeout = 3 * time.Second
	// pdReqTimeout access the request timeout for the pd api service
	pdReqTimeout = 3 * time.Second
	// startTidbTimeout start tidb three sub-module pd / tikv / tidb timeout
	startTidbTimeout = 60 * time.Second
	// storageTimeout data storage timeout
	storageTimeout = 3 * time.Second
)

var (
	// docker
	dockerRegistry string
	k8sAddr        string

	etcdAddress string // storage
	forceInitMd bool
)

func configInit() {
	dr := beego.AppConfig.String("dockerRegistry")
	if dr == "" {
		panic(fmt.Errorf("No set docker registry addr"))
	}
	dockerRegistry = dr

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
}
