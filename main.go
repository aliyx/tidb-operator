package main

import (
	"math/rand"
	"time"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"

	_ "github.com/ffan/tidb-k8s/routers"
	_ "github.com/go-sql-driver/mysql"

	"flag"

	"github.com/ffan/tidb-k8s/models"
	"github.com/ffan/tidb-k8s/pkg/k8sutil"
)

func main() {
	flag.Parse()
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

	rand.Seed(time.Now().Unix())

	k8sutil.CreateNamespace()
	models.Init()

	beego.Run()
}
