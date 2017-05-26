package main

import (
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"

	_ "github.com/ffan/tidb-k8s/routers"
	_ "github.com/go-sql-driver/mysql"

	"github.com/ffan/tidb-k8s/models"
)

func main() {
	logs.SetLogger("console")

	if beego.BConfig.RunMode == "dev" {
		beego.BConfig.WebConfig.DirectoryIndex = true
		beego.BConfig.WebConfig.StaticDir["/swagger"] = "swagger"
	}
	models.Init()
	logs.Debug("==========%s", beego.BConfig.Listen.HTTPAddr)
	beego.Run()
}
