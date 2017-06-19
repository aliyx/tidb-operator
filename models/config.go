package models

import (
	"time"

	"fmt"

	"github.com/astaxie/beego"
	"github.com/ffan/tidb-k8s/pkg/k8sutil"
)

const (
	tableTidb     = "tidbs"
	tableMetadata = "metadata"
	tableEvent    = "events"

	// pdReqTimeout access the request timeout for the pd api service
	pdReqTimeout = 3 * time.Second
)

var (
	etcdAddress   string // storage
	forceInitMd   bool
	imageRegistry string
)

func parseConfig() {
	etcdAddress = beego.AppConfig.String("etcdAddress")
	if etcdAddress == "" {
		ip, err := k8sutil.GetEtcdIP()
		if err != nil {
			panic("cannt get etcd ip")
		}
		etcdAddress = fmt.Sprintf("http://%s:2379", ip)
	}
	forceInitMd = beego.AppConfig.DefaultBool("forceInitMd", false)
	imageRegistry = beego.AppConfig.String("dockerRegistry")
	if imageRegistry == "" {
		panic("cannt get images registry address")
	}
}
