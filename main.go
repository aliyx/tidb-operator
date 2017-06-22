package main

import (
	"os"
	"os/signal"
	"syscall"

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

	k8sutil.Init(beego.AppConfig.String("k8sAddr"))
	models.ParseConfig()
	models.Init()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	go beego.Run()

	sig := <-sc
	logs.Info("Got signal [%d] to exit.", sig)
	switch sig {
	case syscall.SIGTERM:
		os.Exit(0)
	default:
		os.Exit(1)
	}
}
