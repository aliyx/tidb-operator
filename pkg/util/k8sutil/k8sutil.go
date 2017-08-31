package k8sutil

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"time"

	"github.com/ffan/tidb-operator/pkg/spec"

	"github.com/astaxie/beego/logs"

	"fmt"

	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // for gcp auth
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	kubePublicNamespace = "kube-public"
)

var (
	defaultk8sReqTimeout = 3 * time.Second
	defaultImageRegistry = "10.209.224.13:10500/ffan/rds"

	tidbVersionAnnotationKey = "tidb.version"

	masterHost string
	// Namespace all tidb namespace
	Namespace string

	kubecli kubernetes.Interface

	// InCluster is in kubernetes
	InCluster = false

	// ErrNotDeleted ...
	ErrNotDeleted = errors.New("not deleted")
)

func init() {
	Namespace = os.Getenv("MY_NAMESPACE")
	if len(Namespace) == 0 {
		Namespace = "default"
	}
	logs.Info("current namespace is %s", Namespace)
}

// Init create tidb namespace
func Init(addr string) {
	masterHost = addr
	logs.Info("kubernetes master host is %s", masterHost)

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
	InCluster = true
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

// NewRESTClient return a restClient
func NewRESTClient() rest.Interface {
	return kubecli.Core().RESTClient()
}

// NewCRClientWithCodecFactory new cr client with cf
func NewCRClientWithCodecFactory(sc *runtime.Scheme) (*rest.RESTClient, error) {
	config, err := ClusterConfig()
	if err != nil {
		return nil, err
	}

	config.GroupVersion = &spec.SchemeGroupVersion
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: serializer.NewCodecFactory(sc)}

	restcli, err := rest.RESTClientFor(config)
	if err != nil {
		return nil, err
	}
	return restcli, nil
}

// IsKubernetesResourceAlreadyExistError whether it is resource error
func IsKubernetesResourceAlreadyExistError(err error) bool {
	return apierrors.IsAlreadyExists(err)
}

// IsKubernetesResourceNotFoundError whether it is resource not found error
func IsKubernetesResourceNotFoundError(err error) bool {
	return apierrors.IsNotFound(err)
}

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

// DeleteOptions return DeleteOptions with gracePeriodSeconds
func DeleteOptions(gracePeriodSeconds int64) *metav1.DeleteOptions {
	return &metav1.DeleteOptions{
		GracePeriodSeconds: func(t int64) *int64 { return &t }(gracePeriodSeconds),
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

// GetNodesExternalIP get node external IP
func GetNodesExternalIP(selector map[string]string) ([]string, error) {
	option := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(selector).String(),
	}
	nodes, err := kubecli.CoreV1().Nodes().List(option)
	if err != nil {
		return nil, err
	}
	var ips []string
	for _, node := range nodes.Items {
		addrs := node.Status.Addresses
		for _, addr := range addrs {
			if addr.Type == v1.NodeInternalIP {
				ips = append(ips, addr.Address)
				break
			}
		}
	}
	return ips, nil
}

// GetEtcdIP get kubernetes used etcd
func GetEtcdIP() (string, error) {
	ls := map[string]string{
		"component": "etcd",
		"tier":      "control-plane",
	}
	pods, err := GetPodsByNamespace("kube-system", ls)
	if err != nil {
		return "", err
	}
	if len(pods) != 1 {
		return "", fmt.Errorf("get multi etcd %s", GetPodNames(pods))
	}
	return pods[0].Status.PodIP, nil
}

// GetTidbVersion get tidb image version from annotations
func GetTidbVersion(i interface{}) string {
	switch v := i.(type) {
	case *v1.Pod:
		return v.Annotations[tidbVersionAnnotationKey]
	case *v1.ReplicationController:
		return v.Annotations[tidbVersionAnnotationKey]
	}
	return ""
}

// SetTidbVersion set tidb image version
func SetTidbVersion(i interface{}, version string) {
	switch v := i.(type) {
	case *v1.Pod:
		if len(v.Annotations) < 1 {
			v.Annotations = make(map[string]string)
		}
		v.Annotations[tidbVersionAnnotationKey] = version
	case *v1.ReplicationController:
		if len(v.Annotations) < 1 {
			v.Annotations = make(map[string]string)
		}
		v.Annotations[tidbVersionAnnotationKey] = version

		if len(v.Spec.Template.Annotations) < 1 {
			v.Spec.Template.Annotations = make(map[string]string)
		}
		v.Spec.Template.Annotations[tidbVersionAnnotationKey] = version
	}
}

func MakeEmptyDirVolume(name string) v1.Volume {
	return v1.Volume{
		Name:         name,
		VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}},
	}
}

func MakeTZEnvVar() v1.EnvVar {
	return v1.EnvVar{
		Name:  "TZ",
		Value: "Asia/Shanghai",
	}
}

func MakePodIPEnvVar() v1.EnvVar {
	return v1.EnvVar{
		Name: "POD_IP",
		ValueFrom: &v1.EnvVarSource{
			FieldRef: &v1.ObjectFieldSelector{
				FieldPath: "status.podIP",
			},
		},
	}
}

func MakeResourceList(cpu, mem int) v1.ResourceList {
	return v1.ResourceList{
		v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%dm", cpu)),
		v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", mem)),
	}
}
