package k8sutil

import (
	"fmt"
	"strings"
	"time"

	"github.com/ffan/tidb-operator/pkg/spec"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/util/retryutil"

	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

// TODO: replace this package with Operator client

// TidbClusterCRUpdateFunc is a function to be used when atomically
// updating a Cluster CR.

// CreateCRD create a new CRD if no exist
func CreateCRD(kind string) error {
	clientset := mustNewKubeExtClient()
	singular := strings.ToLower(kind)
	crd := &apiextensionsv1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%ss.%s", singular, spec.CRDGroup),
		},
		Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
			Group:   spec.CRDGroup,
			Version: spec.CRDVersion,
			Scope:   apiextensionsv1beta1.NamespaceScoped,
			Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
				Singular: singular,
				Plural:   singular + "s",
				Kind:     kind,
			},
		},
	}
	_, err := clientset.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		return nil
	}
	return WaitCRDReady(kind, clientset)
}

// WaitCRDReady wait CRD create finished
func WaitCRDReady(kind string, clientset apiextensionsclient.Interface) error {
	err := retryutil.Retry(5*time.Second, 20, func() (bool, error) {
		crd, err := clientset.ApiextensionsV1beta1().CustomResourceDefinitions().Get(kind, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, cond := range crd.Status.Conditions {
			switch cond.Type {
			case apiextensionsv1beta1.Established:
				if cond.Status == apiextensionsv1beta1.ConditionTrue {
					return true, nil
				}
			case apiextensionsv1beta1.NamesAccepted:
				if cond.Status == apiextensionsv1beta1.ConditionFalse {
					return false, fmt.Errorf("Name conflict: %v", cond.Reason)
				}
			}
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("wait CRD created failed: %v", err)
	}
	return nil
}

func mustNewKubeExtClient() apiextensionsclient.Interface {
	cfg, err := ClusterConfig()
	if err != nil {
		panic(err)
	}
	return apiextensionsclient.NewForConfigOrDie(cfg)
}

// WatchTidbs watch tidb TPR change
func WatchTidbs(restClient *rest.RESTClient, ns string, resourceVersion string) (watch.Interface, error) {
	uri := fmt.Sprintf("/apis/%s/%s/namespaces/%s/tidbs?watch=true&resourceVersion=%s",
		spec.CRDGroup, spec.CRDVersion, ns, resourceVersion)
	logs.Info("watch tidb uri: %s", uri)
	return restClient.Get().RequestURI(uri).Watch()
}
