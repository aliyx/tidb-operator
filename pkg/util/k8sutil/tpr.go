package k8sutil

import (
	"fmt"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/pkg/spec"
	"github.com/ffan/tidb-k8s/pkg/util/retryutil"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	v1beta1extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/rest"
)

// CreateTPR create a new TPR if no exist
func CreateTPR(name string) error {
	tpr := &v1beta1extensions.ThirdPartyResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s.%s", name, spec.TPRGroup),
		},
		Versions: []v1beta1extensions.APIVersion{
			{Name: spec.TPRVersion},
		},
		Description: spec.TPRDescription,
	}
	_, err := kubecli.ExtensionsV1beta1().ThirdPartyResources().Create(tpr)
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		return nil
	}
	uri := fmt.Sprintf("/apis/%s/%s/namespaces/%s/%ss", spec.TPRGroup, spec.TPRVersion, Namespace, name)
	return WaitTPRReady(kubecli.CoreV1().RESTClient(), 3*time.Second, 30*time.Second, uri)
}

// WaitTPRReady wait TPR create finished
func WaitTPRReady(restcli rest.Interface, interval, timeout time.Duration, uri string) error {
	return retryutil.Retry(interval, int(timeout/interval), func() (bool, error) {
		_, err := restcli.Get().RequestURI(uri).DoRaw()
		if err != nil {
			if apierrors.IsNotFound(err) { // not set up yet. wait more.
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
}

// WatchTidbs watch tidb TPR change
func WatchTidbs(restClient *rest.RESTClient, ns string, resourceVersion string) (watch.Interface, error) {
	uri := fmt.Sprintf("/apis/%s/%s/namespaces/%s/tidbs?watch=true&resourceVersion=%s",
		spec.TPRGroup, spec.TPRVersion, ns, resourceVersion)
	logs.Debug("watch tidb uri: %s", uri)
	return restClient.Get().RequestURI(uri).Watch()
}
