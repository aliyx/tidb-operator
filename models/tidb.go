package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/astaxie/beego/logs"

	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
	"github.com/ffan/tidb-operator/pkg/util/retryutil"

	"sync"

	"github.com/ffan/tidb-operator/pkg/util/httputil"
	"github.com/ghodss/yaml"
)

func (td *Tidb) install() (err error) {
	e := NewEvent(td.Db.Metadata.Name, "tidb", "install")
	td.Db.Status.Phase = PhaseTidbPending
	td.Db.update()
	defer func() {
		ph := PhaseTidbStarted
		if err != nil {
			ph = PhaseTidbStartFailed
		}
		td.Db.Status.Phase = ph
		if uerr := td.Db.update(); uerr != nil {
			logs.Error("update tidb error: %v", uerr)
		}
		e.Trace(err, fmt.Sprintf("Install tidb replicationcontrollers with %d replicas on k8s", td.Replicas))
	}()
	if err = td.createService(); err != nil {
		return err
	}
	if err = td.createReplicationController(); err != nil {
		return err
	}
	// wait tidb started
	if err = td.waitForOk(); err != nil {
		return err
	}
	return nil
}

func (td *Tidb) createService() (err error) {
	j, err := td.toJSONTemplate(tidbServiceYaml)
	if err != nil {
		return err
	}
	srv, err := k8sutil.CreateServiceByJSON(j)
	if err != nil {
		return err
	}
	ps := getProxys()
	for _, py := range ps {
		td.Db.Status.OuterAddresses = append(td.Db.Status.OuterAddresses, fmt.Sprintf("%s:%d", py, srv.Spec.Ports[0].NodePort))
	}
	td.Db.Status.OuterStatusAddresses = append(td.Db.Status.OuterStatusAddresses,
		fmt.Sprintf("%s:%d", ps[0], srv.Spec.Ports[1].NodePort))
	logs.Info("tidb %s mysql address: %s, status address: %s",
		td.Db.Metadata.Name, td.Db.Status.OuterAddresses, td.Db.Status.OuterStatusAddresses)
	return nil
}

func (td *Tidb) createReplicationController() (err error) {
	j, err := td.toJSONTemplate(tidbRcYaml)
	if err != nil {
		return err
	}
	_, err = k8sutil.CreateRcByJSON(j, waitPodRuningTimeout)
	return err
}

func (td *Tidb) toJSONTemplate(temp string) ([]byte, error) {
	r := strings.NewReplacer(
		"{{version}}", td.Version,
		"{{cpu}}", fmt.Sprintf("%v", td.CPU), "{{mem}}", fmt.Sprintf("%v", td.Mem),
		"{{namespace}}", getNamespace(),
		"{{replicas}}", fmt.Sprintf("%v", td.Replicas),
		"{{registry}}", imageRegistry, "{{cell}}", td.Db.Metadata.Name)
	str := r.Replace(temp)
	j, err := yaml.YAMLToJSON([]byte(str))
	if err != nil {
		return nil, err
	}
	return j, nil
}

func (td *Tidb) waitForOk() (err error) {
	logs.Info("waiting for run tidb %s ok...", td.Db.Metadata.Name)
	interval := 3 * time.Second
	sURL := fmt.Sprintf("http://%s/status", td.Db.Status.OuterStatusAddresses[0])
	err = retryutil.Retry(interval, int(waitTidbComponentAvailableTimeout/(interval)), func() (bool, error) {
		if _, err := httputil.Get(sURL, 2*time.Second); err != nil {
			logs.Warn("get tidb status: %v", err)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		logs.Error("wait tidb %s available: %v", td.Db.Metadata.Name, err)
	} else {
		logs.Info("tidb %s ok", td.Db.Metadata.Name)
	}
	return err
}

func (td *Tidb) uninstall() (err error) {
	defer func() {
		td.Db.Status.MigrateState = ""
		td.Db.Status.ScaleState = 0
		td.Db.Status.OuterAddresses = nil
		td.Db.Status.OuterStatusAddresses = nil
		if err != nil {
			err = td.Db.update()
		}
	}()
	if err = k8sutil.DelRc(fmt.Sprintf("tidb-%s", td.Db.Metadata.Name)); err != nil {
		return err
	}
	if err = k8sutil.DelSrvs(fmt.Sprintf("tidb-%s", td.Db.Metadata.Name)); err != nil {
		return err
	}
	return err
}

func (db *Db) scaleTidbs(replica int, wg *sync.WaitGroup) {
	if replica < 1 {
		return
	}
	if replica == db.Tidb.Replicas {
		return
	}
	wg.Add(1)
	go func() {
		db.Lock()
		defer func() {
			db.Unlock()
			wg.Done()
		}()
		var err error
		e := NewEvent(db.Metadata.Name, "tidb", "scale")
		defer func(r int) {
			if err != nil {
				db.Status.ScaleState |= tidbScaleErr
			}
			db.update()
			e.Trace(err, fmt.Sprintf(`Scale tidb "%s" replica: %d->%d`, db.Metadata.Name, r, replica))
		}(db.Tidb.Replicas)
		md := getCachedMetadata()
		if replica > md.Spec.Tidb.Max {
			err = fmt.Errorf("the replicas of tidb exceeds max %d", md.Spec.Tidb.Max)
			return
		}
		if replica > db.Tidb.Replicas*3 || db.Tidb.Replicas > replica*3 {
			err = fmt.Errorf("each scale can not more or less then 2 times")
			return
		}
		old := db.Tidb.Replicas
		db.Tidb.Replicas = replica
		if err = db.Tidb.validate(); err != nil {
			db.Tidb.Replicas = old
			return
		}
		logs.Info(`scale "tidb-%s" from %d to %d`, db.Metadata.Name, old, replica)
		if err = k8sutil.ScaleReplicationController(fmt.Sprintf("tidb-%s", db.Metadata.Name), replica); err != nil {
			return
		}
	}()
}

func (td *Tidb) isNil() bool {
	return td.Replicas < 1
}

func (td *Tidb) isOk() bool {
	if td.Db.Status.Phase < PhaseTidbStarted || td.Db.Status.Phase > PhaseTidbInited {
		return false
	}
	return true
}
