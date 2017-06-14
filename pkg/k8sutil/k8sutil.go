package k8sutil

import (
	"encoding/json"
	"net"
	"os"
	"time"

	"github.com/astaxie/beego/logs"

	"github.com/astaxie/beego"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // for gcp auth
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	defaultk8sReqTimeout = 3 * time.Second
	defaultImageRegistry = "10.209.224.13:10500/ffan/rds"

	masterHost string
	// Namespace all tidb namespace
	Namespace string

	kubecli kubernetes.Interface
)

func init() {
	masterHost = beego.AppConfig.String("k8sAddr")
	logs.Info("kubernetes master host is %s", masterHost)
	Namespace = os.Getenv("MY_NAMESPACE")
	if len(Namespace) == 0 {
		Namespace = "default"
	}
	logs.Info("current namespace is %s", Namespace)
}

// CreateNamespace create tidb namespace
func CreateNamespace() {
	kubecli = MustNewKubeClient()
	if err := createNamespace(Namespace); err != nil && !IsKubernetesResourceAlreadyExistError(err) {
		logs.Critical("Init k8s namespace %s error: %v", Namespace, err)
		panic(err)
	}
}

// MustNewKubeClient create kube client
func MustNewKubeClient() kubernetes.Interface {
	cfg, err := ClusterConfig()
	if err != nil {
		panic(err)
	}
	return kubernetes.NewForConfigOrDie(cfg)
}

// ClusterConfig compatible with both in and out modes
func ClusterConfig() (*rest.Config, error) {
	if len(masterHost) > 0 {
		return clientcmd.BuildConfigFromFlags(masterHost, "")
	}
	return inClusterConfig()
}

func inClusterConfig() (*rest.Config, error) {
	// Work around https://github.com/kubernetes/kubernetes/issues/40973
	// See https://github.com/coreos/etcd-operator/issues/731#issuecomment-283804819
	if len(os.Getenv("KUBERNETES_SERVICE_HOST")) == 0 {
		addrs, err := net.LookupHost("kubernetes.default.svc")
		if err != nil {
			panic(err)
		}
		os.Setenv("KUBERNETES_SERVICE_HOST", addrs[0])
	}
	if len(os.Getenv("KUBERNETES_SERVICE_PORT")) == 0 {
		os.Setenv("KUBERNETES_SERVICE_PORT", "443")
	}
	return rest.InClusterConfig()
}

// func NewTPRClient() (*rest.RESTClient, error) {
// 	config, err := ClusterConfig()
// 	if err != nil {
// 		return nil, err
// 	}

// 	config.GroupVersion = &schema.GroupVersion{
// 		Group:   spec.TPRGroup,
// 		Version: spec.TPRVersion,
// 	}
// 	config.APIPath = "/apis"
// 	config.ContentType = runtime.ContentTypeJSON
// 	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: api.Codecs}

// 	restcli, err := rest.RESTClientFor(config)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return restcli, nil
// }

// IsKubernetesResourceAlreadyExistError whether it is resource error
func IsKubernetesResourceAlreadyExistError(err error) bool {
	return apierrors.IsAlreadyExists(err)
}

// IsKubernetesResourceNotFoundError whether it is resource not found error
func IsKubernetesResourceNotFoundError(err error) bool {
	return apierrors.IsNotFound(err)
}

// ClusterListOpt We are using internal api types for cluster related.
// func ClusterListOpt(clusterName string) metav1.ListOptions {
// 	return metav1.ListOptions{
// 		LabelSelector: labels.SelectorFromSet(LabelsForCluster(clusterName)).String(),
// 	}
// }

// func LabelsForCluster(clusterName string) map[string]string {
// 	return map[string]string{
// 		"etcd_cluster": clusterName,
// 		"app":          "etcd",
// 	}
// }

// CreatePatch creata a patch
func CreatePatch(o, n, datastruct interface{}) ([]byte, error) {
	oldData, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	newData, err := json.Marshal(n)
	if err != nil {
		return nil, err
	}
	return strategicpatch.CreateTwoWayMergePatch(oldData, newData, datastruct)
}

// CascadeDeleteOptions return DeleteOptions with cascade
func CascadeDeleteOptions(gracePeriodSeconds int64) *metav1.DeleteOptions {
	return &metav1.DeleteOptions{
		GracePeriodSeconds: func(t int64) *int64 { return &t }(gracePeriodSeconds),
		PropagationPolicy: func() *metav1.DeletionPropagation {
			foreground := metav1.DeletePropagationForeground
			return &foreground
		}(),
	}
}

// mergeLables merges l2 into l1. Conflicting label will be skipped.
func mergeLabels(l1, l2 map[string]string) {
	for k, v := range l2 {
		if _, ok := l1[k]; ok {
			continue
		}
		l1[k] = v
	}
}