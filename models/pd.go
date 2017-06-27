package models

import (
	"fmt"
	"time"

	"strings"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
	"github.com/ffan/tidb-operator/pkg/util/pdutil"
	"github.com/ffan/tidb-operator/pkg/util/retryutil"
	"github.com/ghodss/yaml"
	"k8s.io/client-go/pkg/api/v1"
)

func (p *Pd) uninstall() (err error) {
	e := NewEvent(p.Db.Metadata.Name, "pd", "uninstall")
	defer func() {
		p.Member = 0
		p.InnerAddresses = nil
		p.OuterAddresses = nil
		p.Db.update()
		if uerr := p.Db.update(); uerr != nil {
			logs.Error("update tidb error: %v", uerr)
		}
		e.Trace(err, "Uninstall pd pods")
	}()
	if err = k8sutil.DeletePodsBy(p.Db.Metadata.Name, "pd"); err != nil {
		return err
	}
	if err = k8sutil.DelSrvs(fmt.Sprintf("pd-%s", p.Db.Metadata.Name),
		fmt.Sprintf("pd-%s-srv", p.Db.Metadata.Name)); err != nil {
		return err
	}
	return err
}

func (p *Pd) install() (err error) {
	e := NewEvent(p.Db.Metadata.Name, "pd", "install")
	p.Db.Status.Phase = PhasePdPending
	if err = p.Db.update(); err != nil {
		e.Trace(err,
			fmt.Sprintf("Update tidb status to %d error: %v", PhasePdPending, err))
		return err
	}
	defer func() {
		ph := PhasePdStarted
		if err != nil {
			ph = PhasePdStartFailed
		}
		p.Db.Status.Phase = ph
		if uerr := p.Db.update(); uerr != nil {
			logs.Error("update tidb error: %v", uerr)
		}
		e.Trace(err,
			fmt.Sprintf("Install pd services and pods with replicas desire: %d, running: %d on k8s", p.Replicas, p.Member))
	}()
	if err = p.createServices(); err != nil {
		return err
	}
	for i := 0; i < p.Spec.Replicas; i++ {
		p.Member++
		if err = p.createPod(); err != nil {
			return err
		}
	}
	// Waiting for pds available
	if err = p.waitForOk(); err != nil {
		return err
	}
	return err
}

func (p *Pd) createServices() error {
	// create headless
	if _, err := p.createService(pdHeadlessServiceYaml); err != nil {
		return err
	}

	// create service
	srv, err := p.createService(pdServiceYaml)
	if err != nil {
		return err
	}
	p.InnerAddresses = append(p.InnerAddresses, fmt.Sprintf("%s:%d", srv.Spec.ClusterIP, srv.Spec.Ports[0].Port))
	ps := getProxys()
	p.OuterAddresses = append(p.OuterAddresses, fmt.Sprintf("%s:%d", ps[0], srv.Spec.Ports[0].NodePort))
	return nil
}

func (p *Pd) createService(temp string) (*v1.Service, error) {
	j, err := p.toJSONTemplate(temp)
	if err != nil {
		return nil, err
	}
	retSrv, err := k8sutil.CreateServiceByJSON(j)
	if err != nil {
		return nil, err
	}
	return retSrv, nil
}

func (p *Pd) createPod() (err error) {
	var j []byte
	if j, err = p.toJSONTemplate(pdPodYaml); err != nil {
		return err
	}
	if _, err = k8sutil.CreateAndWaitPodByJSON(j, waitPodRuningTimeout); err != nil {
		return err
	}
	return nil
}

func (p *Pd) waitForOk() (err error) {
	logs.Info("waiting for run pd %s ok...", p.Db.Metadata.Name)
	interval := 3 * time.Second
	err = retryutil.Retry(interval, int(waitTidbComponentAvailableTimeout/(interval)), func() (bool, error) {
		if _, err = pdutil.PdLeaderGet(p.OuterAddresses[0]); err != nil {
			logs.Warn("get pd leader: %v", err)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		logs.Error("wait pd %s available: %v", p.Db.Metadata.Name, err)
	} else {
		logs.Info("pd %s ok", p.Db.Metadata.Name)
	}
	return err
}

func (p *Pd) toJSONTemplate(temp string) ([]byte, error) {
	r := strings.NewReplacer(
		"{{namespace}}", getNamespace(),
		"{{cell}}", p.Db.Metadata.Name,
		"{{id}}", fmt.Sprintf("%03d", p.Member),
		"{{replicas}}", fmt.Sprintf("%d", p.Spec.Replicas),
		"{{cpu}}", fmt.Sprintf("%v", p.Spec.CPU),
		"{{mem}}", fmt.Sprintf("%v", p.Spec.Mem),
		"{{version}}", p.Spec.Version,
		"{{tidbdata_volume}}", p.Spec.Volume,
		"{{registry}}", imageRegistry,
	)
	str := r.Replace(temp)
	j, err := yaml.YAMLToJSON([]byte(str))
	if err != nil {
		return nil, err
	}
	return j, nil
}

func isOkPd(cell string) bool {
	if db, err := GetDb(cell); err != nil ||
		db == nil || db.Status.Phase < PhasePdStarted || db.Status.Phase > PhaseTidbInited {
		return false
	}
	return true
}

func (p *Pd) isNil() bool {
	return p.Spec.Replicas < 1
}
