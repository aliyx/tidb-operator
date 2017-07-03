package garbagecollection

import (
	"os"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/operator"
	"github.com/ffan/tidb-operator/pkg/spec"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
)

func TestMain(m *testing.M) {
	logs.SetLogFuncCall(true)
	beego.AppConfig.Set("k8sAddr", "http://10.213.44.128:10218")
	operator.Init()
	os.Exit(m.Run())
}

func TestWatcher_Run(t *testing.T) {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)
	AddToScheme(scheme)
	tpr, err := k8sutil.NewTPRClientWithCodecFactory(spec.TPRGroup, spec.TPRVersion, codecs)
	if err != nil {
		t.Error(err)
	}
	hostname, _ := os.Hostname()
	c := Config{
		HostName:      hostname,
		Namespace:     "default",
		PVProvisioner: "local",
		Tprclient:     tpr,
	}
	if err = c.Validate(); err != nil {
		t.Error(err)
	}
	w := NewWatcher(c)
	if err := w.Run(); err != nil {
		t.Error(err)
	}
}
