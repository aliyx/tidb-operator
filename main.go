package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/operator"

	_ "github.com/ffan/tidb-operator/operator/routers"
	_ "github.com/go-sql-driver/mysql"

	"context"
	"flag"
	"strconv"
)

var (
	logLevel       int
	k8sAddress     string
	httpaddr       string
	httpport       int
	enableDocs     bool
	runmode        string
	dockerRegistry string
	forceInitMd    bool
)

func init() {
	flag.StringVar(&httpaddr, "http-addr", "0.0.0.0", "The address on which the HTTP server will listen to.")
	flag.IntVar(&httpport, "http-port", 12808, "The port on which the HTTP server will listen to.")
	flag.BoolVar(&enableDocs, "enable-docs", false, "Enable show swagger.")
	flag.StringVar(&runmode, "runmode", "dev", "run mode, eg: dev test prod.")
	flag.IntVar(&logLevel, "log-level", logs.LevelInfo, "Beego logs level.")
	flag.StringVar(&k8sAddress, "k8s-address", os.Getenv("K8S_ADDRESS"), "Kubernetes api address, if deployed in kubernetes, do not need to set, eg: 'http://127.0.0.1:8080'")
	flag.StringVar(&dockerRegistry, "docker-registry", "10.209.224.13:10500/ffan/rds", "private docker registry.")
	flag.BoolVar(&forceInitMd, "init-md", false, "force init metadata.")

	flag.Parse()

	// set logs

	logs.SetLogger(logs.AdapterConsole)
	logs.SetLogFuncCall(true)
	logs.SetLevel(logLevel)

	// set env

	beego.BConfig.AppName = "tidb-operator"
	// can't get body data,if no set
	beego.BConfig.CopyRequestBody = true
	beego.BConfig.WebConfig.AutoRender = false
	beego.BConfig.WebConfig.EnableDocs = enableDocs
	beego.BConfig.RunMode = runmode
	beego.BConfig.Listen.HTTPAddr = httpaddr
	beego.BConfig.Listen.HTTPPort = httpport
	if len(k8sAddress) > 0 {
		beego.AppConfig.Set("k8sAddr", k8sAddress)
	}
	if len(dockerRegistry) > 0 {
		beego.AppConfig.Set("dockerRegistry", dockerRegistry)
	}
	beego.AppConfig.Set("forceInitMd", strconv.FormatBool(forceInitMd))

	switch beego.BConfig.RunMode {
	case "dev":
		beego.BConfig.WebConfig.DirectoryIndex = true
		beego.BConfig.WebConfig.StaticDir["/swagger"] = "swagger"
	}
}

func main() {
	operator.ParseConfig()

	operator.Init()

	ctx, cancel := context.WithCancel(context.Background())
	err := operator.Run(ctx)
	if err != nil {
		panic(err)
	}

	go beego.Run()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	sig := <-sc
	logs.Info("Got signal [%d] to exit.", sig)
	cancel()
	switch sig {
	case syscall.SIGTERM:
		os.Exit(0)
	default:
		os.Exit(1)
	}
}
