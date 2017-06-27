package garbagecollection

import (
	"os"
	"testing"

	"github.com/astaxie/beego"
	"github.com/ffan/tidb-k8s/models"
	"github.com/ffan/tidb-k8s/pkg/spec"
	"github.com/ffan/tidb-k8s/pkg/util/k8sutil"
)

func TestMain(m *testing.M) {
	beego.AppConfig.Set("k8sAddr", "http://10.213.44.128:10218")
	models.Init()
	os.Exit(m.Run())
}

func TestWatcher_Run(t *testing.T) {
	tpr, err := k8sutil.NewTPRClient(spec.TPRGroup, spec.TPRVersion)
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
	w := New(c)
	if err := w.Run(); err != nil {
		t.Error(err)
	}
}
