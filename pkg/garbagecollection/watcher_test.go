package garbagecollection

import (
	"os"
	"testing"

	"github.com/astaxie/beego"
	"github.com/ffan/tidb-k8s/models"
	"github.com/ffan/tidb-k8s/pkg/util/k8sutil"
)

func TestMain(m *testing.M) {
	beego.AppConfig.Set("k8sAddr", "http://10.213.44.128:10218")
	models.Init()
	os.Exit(m.Run())
}

func TestWatcher_Run(t *testing.T) {
	w := New(Config{
		Namespace:     "default",
		PVProvisioner: "local",
		KubeCli:       k8sutil.MustNewKubeClient(),
	})
	if err := w.Run(); err != nil {
		t.Error(err)
	}
}
