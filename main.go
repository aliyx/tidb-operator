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
	switch beego.BConfig.RunMode {
	case "dev":
		beego.BConfig.WebConfig.DirectoryIndex = true
		beego.BConfig.WebConfig.StaticDir["/swagger"] = "swagger"
	case "test":
		beego.SetLevel(beego.LevelInformational)
	case "pord":
		beego.SetLevel(beego.LevelInformational)
	}
	models.Init()
	beego.Run()
}
