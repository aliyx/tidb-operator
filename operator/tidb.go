package operator

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/client-go/pkg/api/v1"

	"github.com/astaxie/beego/logs"

	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
	"github.com/ffan/tidb-operator/pkg/util/retryutil"

	"sync"

	"github.com/ffan/tidb-operator/pkg/util/httputil"
	"github.com/ghodss/yaml"
)

var (
	scaleMu sync.Mutex
)

func (td *Tidb) upgrade() (err error) {
	if td.Db.Status.Phase < PhaseTidbStarted {
		return fmt.Errorf("the db %s tidb unavailable", td.Db.Metadata.Name)
	}

	var (
		upgraded bool
		count    int
		pods     []string
	)

	e := NewEvent(td.Db.Metadata.Name, "tidb/tidb", "upgrate")
	defer func() {
		// have upgrade
		if count > 0 || err != nil {
			e.Trace(err, fmt.Sprintf("upgrate tidb to version: %s", td.Version))
		}
	}()

	// get tidb pods name
	pods, err = k8sutil.ListPodNames(td.Db.Metadata.Name, "tidb")
	if err != nil {
		return err
	}
	for _, podName := range pods {
		upgraded, err = upgradeOne(podName, fmt.Sprintf("%s/tidb:%s", imageRegistry, td.Version), td.Version)
		if err != nil {
			return err
		}
		if upgraded {
			count++
			time.Sleep(reconcileInterval)
		}
	}
	return nil
}

func (td *Tidb) install() (err error) {
	e := NewEvent(td.Db.Metadata.Name, "tidb/tidb", "install")
	td.Db.Status.Phase = PhaseTidbPending
	td.Db.update()

	defer func() {
		parseError(td.Db, err)
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
		td.Db.Status.OuterAddresses =
			append(td.Db.Status.OuterAddresses, fmt.Sprintf("%s:%d", py, srv.Spec.Ports[0].NodePort))
	}
	td.Db.Status.OuterStatusAddresses =
		append(td.Db.Status.OuterStatusAddresses, fmt.Sprintf("%s:%d", ps[0], srv.Spec.Ports[1].NodePort))
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

	td := db.Tidb
	if replica == td.Replicas {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := td.reconcile(); err != nil {
				logs.Error("failed to reconcile tidb %s %v", db.GetName(), err)
			}
		}()
		return
	}
	wg.Add(1)
	go func() {
		scaleMu.Lock()
		defer func() {
			wg.Done()
			scaleMu.Unlock()
		}()
		var err error
		e := NewEvent(db.Metadata.Name, "tidb/tidb", "scale")
		defer func(r int) {
			parseError(db, err)
			if err != nil {
				db.Status.ScaleState |= tidbScaleErr
			}
			if uerr := db.update(); uerr != nil {
				logs.Error("failed to update db %s %v", db.Metadata.Name, uerr)
			}
			e.Trace(err, fmt.Sprintf("Scale tidb '%s' replicas: %d -> %d", db.Metadata.Name, r, replica))
		}(td.Replicas)
		md := getCachedMetadata()
		if replica > md.Spec.Tidb.Max {
			err = fmt.Errorf("the replicas of tidb exceeds max %d", md.Spec.Tidb.Max)
			return
		}
		if replica > td.Replicas*3 || td.Replicas > replica*3 {
			err = fmt.Errorf("each scale can not more or less then 2 times")
			return
		}
		if replica < 1 {
			err = fmt.Errorf("replicas must be greater than 1")
			return
		}
		logs.Info("start scaling tidb count of the db '%s' from %d to %d",
			db.Metadata.Name, td.Replicas, replica)
		td.Replicas = replica
		if err = k8sutil.ScaleReplicationController(fmt.Sprintf("tidb-%s", db.Metadata.Name), replica); err != nil {
			return
		}
	}()
}

// delete invalid pods
func (td *Tidb) reconcile() error {
	var (
		err  error
		pods []v1.Pod
	)
	pods, err = k8sutil.GetPods(td.Db.Metadata.Name, "tidb")
	if err != nil {
		return err
	}
	for _, pod := range pods {
		if pod.Status.Phase != v1.PodRunning {
			if err = k8sutil.DeletePods(pod.Name); err != nil {
				return err
			}
		}
	}
	return nil
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
