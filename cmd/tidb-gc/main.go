package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"strings"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/garbagecollection"
	"github.com/ffan/tidb-operator/operator"
	"github.com/ffan/tidb-operator/pkg/spec"
	"github.com/ffan/tidb-operator/pkg/util/constants"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

var (
	logLevel   int
	k8sAddress string
	exclude    string
)

func init() {
	flag.IntVar(&logLevel, "log-level", logs.LevelDebug, "Beego logs level.")
	flag.StringVar(&k8sAddress, "k8s-address", "", "Kubernetes api address, if deployed in kubernetes, do not need to set.")
	flag.StringVar(&exclude, "exclude", "grafana,prometheus", "Exclude which files to be recycled.")
	flag.Parse()

	// set logs

	logs.SetLogger("console")
	logs.SetLogFuncCall(true)
	logs.SetLevel(logs.LevelInfo)

	// set env
	beego.AppConfig.Set("k8sAddr", k8sAddress)
}

func main() {
	// get node name
	var err error
	node := os.Getenv("NODE_NAME")
	if node == "" {
		// for test
		node, err = os.Hostname()
		if err != nil {
			panic(fmt.Sprintf("get nodeName: %v", err))
		}
	}

	// register schema for serializer

	operator.Init()
	scheme := runtime.NewScheme()
	scheme.AddUnversionedTypes(apiv1.SchemeGroupVersion, &metav1.Status{})
	codecs := serializer.NewCodecFactory(scheme)
	garbagecollection.AddToScheme(scheme)

	tpr, err := k8sutil.NewTPRClientWithCodecFactory(spec.TPRGroup, spec.TPRVersion, codecs)
	if err != nil {
		panic(fmt.Sprintf("create a tpr client: %v", err))
	}
	c := garbagecollection.Config{
		HostName:      node,
		Namespace:     k8sutil.Namespace,
		PVProvisioner: constants.PVProvisionerHostpath,
		Tprclient:     tpr,
		ExcludeFiles:  strings.Split(exclude, ","),
	}
	if err = c.Validate(); err != nil {
		panic(fmt.Sprintf("validate config: %v", err))
	}
	w := garbagecollection.NewWatcher(c)

	// clear all metrics by restart a new Pod 'prom-gateway'

	pods, err := k8sutil.GetPodsByNamespace(k8sutil.Namespace, map[string]string{"name": "prom-gateway"})
	if err != nil || len(pods) != 1 {
		logs.Error("can't get Pod 'prom-gateway'", err)
	} else {
		if pods[0].Spec.NodeName == node {
			if err = k8sutil.DeletePod(pods[0].GetName(), 6); err != nil {
				logs.Error("delete prom-gateway failed, please delete manually %v", err)
			} else {
				logs.Info("prom-gateway deleted, kubernetes will create a new Pod")
			}
		}
	}

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

	if err = w.Run(); err != nil {
		panic(fmt.Sprintf("run garbage collection watcher: %v", err))
	}
}
