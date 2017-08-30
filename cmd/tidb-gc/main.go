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
	"github.com/ffan/tidb-operator/pkg/util/constants"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

	operator.Init()

	// register schema for serializer

	scheme := runtime.NewScheme()
	scheme.AddUnversionedTypes(v1.SchemeGroupVersion, &metav1.Status{})
	operator.AddToScheme(scheme)
	tpr, err := k8sutil.NewCRClientWithCodecFactory(scheme)
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
	pgName := "prom-gateway"
	pods, err := k8sutil.GetPodsByNamespace(k8sutil.Namespace, map[string]string{"name": pgName})
	if err != nil {
		logs.Error("could not get %q pods for to clear all deleted metrics", pgName, err)
	} else if len(pods) > 0 {
		for _, pod := range pods {
			if pod.Spec.NodeName == node {
				if err = k8sutil.DeletePod(pod.GetName(), 6); err != nil {
					logs.Error("failed to delete Pod %q, please delete manually %v", pod.GetName(), err)
				} else {
					logs.Info("Pod %q deleted, kubernetes will create a new Pod", pod.GetName())
				}
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
