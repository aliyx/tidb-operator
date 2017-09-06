package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/operator"

	_ "github.com/ffan/tidb-operator/operator/routers"
	_ "github.com/go-sql-driver/mysql"

	"k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"

	"context"
	"flag"
)

var (
	defaultHostPath = "/mnt"
	defaultMount    = "data"

	hostPath string
	mount    string

	logLevel       int
	k8sAddress     string
	httpaddr       string
	httpport       int
	enableDocs     bool
	runmode        string
	dockerRegistry string
	forceInitMd    bool

	gcName    = "tidb-gc"
	namespace string
)

func init() {
	flag.StringVar(&httpaddr, "http-addr", "0.0.0.0", "The address on which the HTTP server will listen to.")
	flag.IntVar(&httpport, "http-port", 12808, "The port on which the HTTP server will listen to.")
	flag.BoolVar(&enableDocs, "enable-docs", true, "Enable show swagger.")
	flag.StringVar(&runmode, "runmode", "dev", "run mode, eg: dev test prod.")
	flag.IntVar(&logLevel, "log-level", logs.LevelDebug, "Beego logs level.")
	flag.StringVar(&k8sAddress, "k8s-address", os.Getenv("K8S_ADDRESS"), "Kubernetes api address, if deployed in kubernetes, do not need to set, eg: 'http://127.0.0.1:8080'")
	flag.StringVar(&dockerRegistry, "docker-registry", "10.209.224.13:10500/ffan/rds", "private docker registry.")
	flag.BoolVar(&forceInitMd, "init-md", false, "Force init metadata.")
	flag.StringVar(&hostPath, "host-path", defaultHostPath, "The tikv hostPath volume.")
	flag.StringVar(&mount, "mount", defaultMount, "The path prefix of tikv mount.")

	flag.Parse()

	// set logs

	logs.SetLogger(logs.AdapterConsole)
	logs.SetLogFuncCall(true)
	logs.SetLevel(logLevel)

	// set beego

	beego.BConfig.AppName = "tidb-operator"
	// can't get body data,if no set
	beego.BConfig.CopyRequestBody = true
	beego.BConfig.WebConfig.AutoRender = false
	beego.BConfig.WebConfig.EnableDocs = enableDocs
	beego.BConfig.RunMode = runmode
	beego.BConfig.Listen.HTTPAddr = httpaddr
	beego.BConfig.Listen.HTTPPort = httpport
	switch beego.BConfig.RunMode {
	case "dev":
		beego.BConfig.WebConfig.DirectoryIndex = true
		beego.BConfig.WebConfig.StaticDir["/swagger"] = "swagger"
	}

	operator.ImageRegistry = dockerRegistry
	operator.ForceInitMd = forceInitMd
	operator.HostPath = hostPath
	operator.Mount = mount
	logs.Info("force init metadata:", forceInitMd)
	logs.Info("docker image registrey:", dockerRegistry)
	logs.Info("host path:", hostPath)
	logs.Info("mount:", mount)

	namespace = os.Getenv("MY_NAMESPACE")
	if len(namespace) == 0 {
		namespace = "default"
	}
	k8sutil.Namespace = namespace
	logs.Info("current namespace is %q", namespace)
}

func main() {
	id, err := os.Hostname()
	if err != nil {
		logrus.Fatalf("failed to get hostname: %v", err)
	}
	name := os.Getenv("MY_POD_NAME")
	if len(name) == 0 {
		name = id
	}

	k8sutil.MustInit(k8sAddress)

	startTidbFullGC()

	kubecli := k8sutil.MustNewKubeClient()

	rl, err := resourcelock.New(resourcelock.EndpointsResourceLock,
		namespace,
		"tidb-operator",
		kubecli.(*kubernetes.Clientset),
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: createRecorder(kubecli, name, namespace),
		})
	if err != nil {
		logrus.Fatalf("error creating lock: %v", err)
	}

	leaderelection.RunOrDie(leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				logrus.Fatalf("leader election lost")
			},
		},
	})

	panic("unreachable")
}

func createRecorder(kubecli kubernetes.Interface, name, namespace string) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(logrus.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(kubecli.Core().RESTClient()).Events(namespace)})
	return eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: name})
}

func run(stop <-chan struct{}) {
	operator.Init()
	ctx, cancel := context.WithCancel(context.Background())
	err := operator.Run(ctx)
	if err != nil {
		logrus.Fatalf("failed to run operator: %v", err)
	}

	// start restful api server
	go beego.Run()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc)
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

func startTidbFullGC() {
	var err error
	if err = k8sutil.CreateServiceAccount(gcName); err != nil && !apierrors.IsAlreadyExists(err) {
		logrus.Fatalf("Unable to create service account: %v", err)
	}
	if err = k8sutil.CreateClusterRoleBinding(gcName); err != nil && !apierrors.IsAlreadyExists(err) {
		logrus.Fatalf("Unable to create cluster role bindings: %v", err)
	}
	if err = createDaemonSet(); err != nil && !apierrors.IsAlreadyExists(err) {
		logrus.Fatalf("Unable to create daemonset: %v", err)
	}
}

func createDaemonSet() error {
	envVars := []v1.EnvVar{
		k8sutil.MakeTZEnvVar(),
		{
			Name: "NODE_NAME",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
		{
			Name: "MY_NAMESPACE",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
	}
	containers := []v1.Container{
		v1.Container{
			Name:            gcName,
			Image:           dockerRegistry + "/tidb-gc:latest",
			ImagePullPolicy: v1.PullAlways,
			Resources: v1.ResourceRequirements{
				Limits: k8sutil.MakeResourceList(100, 128),
			},
			VolumeMounts: []v1.VolumeMount{
				{Name: "datadir", MountPath: "/host"},
			},
			Env:     envVars,
			Command: []string{"bash", "-c", "tidb-gc"},
		},
	}

	daemonSet := extv1beta1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gcName,
			Namespace: namespace,
		},
		Spec: extv1beta1.DaemonSetSpec{
			UpdateStrategy: extv1beta1.DaemonSetUpdateStrategy{
				Type: extv1beta1.RollingUpdateDaemonSetStrategyType,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"name": gcName},
				},
				Spec: v1.PodSpec{
					TerminationGracePeriodSeconds: operator.GetTerminationGracePeriodSeconds(),
					Volumes: []v1.Volume{
						{
							Name: "datadir",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: hostPath,
								},
							},
						},
					},
					ServiceAccountName: gcName,
					RestartPolicy:      v1.RestartPolicyAlways,
					Containers:         containers,
				},
			},
		},
	}

	_, err := k8sutil.CreateDaemonset(&daemonSet)
	return err
}
