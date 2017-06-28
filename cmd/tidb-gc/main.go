package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/models"
	"github.com/ffan/tidb-operator/pkg/garbagecollection"
	"github.com/ffan/tidb-operator/pkg/spec"
	"github.com/ffan/tidb-operator/pkg/util/constants"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

func main() {
	logs.SetLogger("console")

	beego.AppConfig.Set("k8sAddr", os.Getenv("K8S_ADDRESS"))

	models.Init()
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)
	garbagecollection.AddToScheme(scheme)
	tpr, err := k8sutil.NewTPRClientWithCodecFactory(spec.TPRGroup, spec.TPRVersion, codecs)
	if err != nil {
		panic(fmt.Sprintf("create a tpr client: %v", err))
	}
	c := garbagecollection.Config{
		Namespace:     k8sutil.Namespace,
		PVProvisioner: constants.PVProvisionerHostpath,
		Tprclient:     tpr,
	}
	if err = c.Validate(); err != nil {
		panic(fmt.Sprintf("validate config: %v", err))
	}
	w := garbagecollection.NewWatcher(c)

	sc := make(chan os.Signal, 1)
	signal.Notify(sc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	go func() {
		sig := <-sc
		logs.Info("Got signal [%d] to exit.", sig)
		switch sig {
		case syscall.SIGTERM:
			os.Exit(0)
		default:
			os.Exit(1)
		}
	}()

	if err := w.Run(); err != nil {
		panic(fmt.Sprintf("run garbage collection watcher: %v", err))
	}
}
