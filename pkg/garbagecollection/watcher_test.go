package garbagecollection

import (
	"os"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/astaxie/beego"
	"github.com/ffan/tidb-operator/models"
	"github.com/ffan/tidb-operator/pkg/spec"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
)

func TestMain(m *testing.M) {
	beego.AppConfig.Set("k8sAddr", "http://10.213.44.128:10218")
	models.Init()
	os.Exit(m.Run())
}

func TestWatcher_Run(t *testing.T) {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)
	addToScheme(scheme)
	tpr, err := k8sutil.NewTPRClientWithCodecFactory(spec.TPRGroup, spec.TPRVersion, codecs)
	if err != nil {
		t.Error(err)
	}
	c := Config{
		Namespace:     "default",
		PVProvisioner: "local",
		tprclient:     tpr,
	}
	if err = c.Validate(); err != nil {
		t.Error(err)
	}
	w := NewWatcher(c)
	if err := w.Run(); err != nil {
		t.Error(err)
	}
}
