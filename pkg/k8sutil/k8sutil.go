package k8sutil

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/coreos/etcd-operator/pkg/spec"
	"github.com/ffan/tidb-k8s/pkg/httputil"

	"github.com/astaxie/beego"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	appsv1beta1 "k8s.io/client-go/pkg/apis/apps/v1beta1"
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

	r := strings.NewReplacer("{{namespace}}", Namespace)
	s := r.Replace(tidbNamespaceYaml)
	if err := createNamespace(s); err != nil {
		if err == httputil.ErrAlreadyExists {
			logs.Warn(`Namespace "%s" already exists`, Namespace)
		} else {
			logs.Critical("Init k8s namespace %s error: %v", Namespace, err)
			panic(err)
		}
	}
}

// GetPodNames get specified pods name
func GetPodNames(pods []*v1.Pod) []string {
	if len(pods) == 0 {
		return nil
	}
	res := []string{}
	for _, p := range pods {
		res = append(res, p.Name)
	}
	return res
}

// PodWithNodeSelector set pod nodeselecter
func PodWithNodeSelector(p *v1.Pod, ns map[string]string) *v1.Pod {
	p.Spec.NodeSelector = ns
	return p
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

func NewTPRClient() (*rest.RESTClient, error) {
	config, err := ClusterConfig()
	if err != nil {
		return nil, err
	}

	config.GroupVersion = &schema.GroupVersion{
		Group:   spec.TPRGroup,
		Version: spec.TPRVersion,
	}
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: api.Codecs}

	restcli, err := rest.RESTClientFor(config)
	if err != nil {
		return nil, err
	}
	return restcli, nil
}

func IsKubernetesResourceAlreadyExistError(err error) bool {
	return apierrors.IsAlreadyExists(err)
}

func IsKubernetesResourceNotFoundError(err error) bool {
	return apierrors.IsNotFound(err)
}

// We are using internal api types for cluster related.
func ClusterListOpt(clusterName string) metav1.ListOptions {
	return metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(LabelsForCluster(clusterName)).String(),
	}
}

func LabelsForCluster(clusterName string) map[string]string {
	return map[string]string{
		"etcd_cluster": clusterName,
		"app":          "etcd",
	}
}

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

func ClonePod(p *v1.Pod) *v1.Pod {
	np, err := api.Scheme.DeepCopy(p)
	if err != nil {
		panic("cannot deep copy pod")
	}
	return np.(*v1.Pod)
}

func cloneDeployment(d *appsv1beta1.Deployment) *appsv1beta1.Deployment {
	cd, err := api.Scheme.DeepCopy(d)
	if err != nil {
		panic("cannot deep copy pod")
	}
	return cd.(*appsv1beta1.Deployment)
}

func PatchDeployment(kubecli kubernetes.Interface, namespace, name string, updateFunc func(*appsv1beta1.Deployment)) error {
	od, err := kubecli.AppsV1beta1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	nd := cloneDeployment(od)
	updateFunc(nd)
	patchData, err := CreatePatch(od, nd, appsv1beta1.Deployment{})
	if err != nil {
		return err
	}
	_, err = kubecli.AppsV1beta1().Deployments(namespace).Patch(name, types.StrategicMergePatchType, patchData)
	return err
}

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

func setLabelSelector(params url.Values, cell, component string) {
	params.Add("labelSelector", fmt.Sprintf("app=tidb,cell=%s,component=%s", cell, component))
}
