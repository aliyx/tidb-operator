package operator

import (
	"fmt"
	"time"

	"strings"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
	"github.com/ffan/tidb-operator/pkg/util/pdutil"
	"github.com/ffan/tidb-operator/pkg/util/retryutil"
	"github.com/ghodss/yaml"
	"github.com/tidwall/gjson"
	"k8s.io/client-go/pkg/api/v1"
)

func (p *Pd) upgrade() error {
	if len(p.Members) < 1 {
		return nil
	}
	if p.Db.Status.Phase < PhasePdStarted {
		return fmt.Errorf("the tidb %s pd unavailable", p.Db.Metadata.Name)
	}

	var (
		err      error
		upgraded bool
		count    int
	)

	e := NewEvent(p.Db.Metadata.Name, "tidb/pd", "upgrate")
	defer func() {
		// have upgrade
		if count > 0 || err != nil {
			e.Trace(err, fmt.Sprintf("upgrate pd to version: %s", p.Version))
		}
	}()

	for _, mb := range p.Members {
		upgraded, err = upgradeOne(mb.Name, fmt.Sprintf("%s/pd:%s", imageRegistry, p.Version), p.Version)
		if err != nil {
			mb.State = PodOffline
			return err
		}
		if upgraded {
			if err = p.waitForOk(); err != nil {
				mb.State = PodOffline
				return err
			}
			count++
			time.Sleep(upgradeInterval)
		}
	}
	return nil
}

func (db *Db) reconcilePds() error {

	var (
		err     error
		p       = db.Pd
		changed = 0
		pods    []v1.Pod
	)

	e := NewEvent(db.GetName(), "tidb/pd", "reconcile")
	defer func() {
		parseError(db, err)
		if changed > 0 || err != nil {
			if err != nil {
				logs.Error("reconcile pd %s error: %v", db.GetName(), err)
			}
			e.Trace(err, "Reconcile pd")
		} else {
			// check version
			for i := range pods {
				pod := pods[i]
				if needUpgrade(&pod, p.Version) {
					if err = p.upgrade(); err != nil {
						return
					}
				}
			}
		}
	}()

	pods, err = k8sutil.GetPods(db.GetName(), "pd")
	if err != nil {
		return err
	}

	// check not running pd member
	for _, mb := range p.Members {
		st := PodOffline
		for _, pod := range pods {
			if pod.GetName() == mb.Name && k8sutil.IsPodOk(pod) {
				st = PodOnline
				break
			}
		}
		mb.State = st
	}

	// delete offline pd and create a new pd
	for i, mb := range p.Members {
		if mb.State == PodOffline {
			changed++
			if err = k8sutil.DeletePods(mb.Name); err != nil {
				return err
			}
			p.Member = i + 1
			if err = p.createPod(); err != nil {
				mb.State = PodOffline
				return err
			}
			mb.State = PodOnline
		}
	}

	if err = p.waitForOk(); err != nil {
		return err
	}

	return nil
}

func (p *Pd) uninstall() (err error) {
	if err = k8sutil.DeletePodsBy(p.Db.GetName(), "pd"); err != nil {
		return err
	}
	if err = k8sutil.DelSrvs(
		fmt.Sprintf("pd-%s", p.Db.GetName()),
		fmt.Sprintf("pd-%s-srv", p.Db.GetName())); err != nil {
		return err
	}
	p.Member = 0
	p.InnerAddresses = nil
	p.OuterAddresses = nil
	p.Members = nil
	return nil
}

func (p *Pd) install() (err error) {
	e := NewEvent(p.Db.GetName(), "tidb/pd", "install")
	p.Db.Status.Phase = PhasePdPending
	if err = p.Db.update(); err != nil {
		e.Trace(err,
			fmt.Sprintf("Update db status to %d error: %v", PhasePdPending, err))
		return err
	}

	defer func() {
		ph := PhasePdStarted
		if err != nil {
			ph = PhasePdStartFailed
		}
		p.Db.Status.Phase = ph
		e.Trace(err,
			fmt.Sprintf("Install pd services and pods with replicas desire: %d, running: %d on k8s", p.Replicas, p.Member))
	}()

	if err = p.createServices(); err != nil {
		return err
	}
	for i := 0; i < p.Replicas; i++ {
		p.Member++
		st := PodOnline
		if err = p.createPod(); err != nil {
			st = PodOffline
		}
		m := &Member{
			Name:  fmt.Sprintf("pd-%s-%03d", p.Db.GetName(), p.Member),
			State: st,
		}
		p.Members = append(p.Members, m)
		if err != nil {
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

func (p *Pd) createPod() error {
	var (
		err error
		b   []byte
	)
	if b, err = p.toJSONTemplate(pdPodYaml); err != nil {
		return err
	}
	if _, err = k8sutil.CreateAndWaitPodByJSON(b, waitPodRuningTimeout); err != nil {
		return err
	}
	return nil
}

func (p *Pd) waitForOk() (err error) {
	logs.Debug("wait for pd %s running", p.Db.GetName())
	interval := 3 * time.Second
	err = retryutil.Retry(interval, int(waitTidbComponentAvailableTimeout/(interval)), func() (bool, error) {
		if _, err = pdutil.PdLeaderGet(p.OuterAddresses[0]); err != nil {
			logs.Warn("get pd %s leader error: %v", p.Db.GetName(), err)
			return false, nil
		}
		js, err := pdutil.PdMembersGet(p.OuterAddresses[0])
		if err != nil {
			return false, err
		}
		ret := gjson.Get(js, "members.#.name")
		if ret.Type == gjson.Null {
			logs.Warn("cann't get pd %s members", p.Db.GetName())
			return false, nil
		}
		if len(ret.Array()) != len(p.Members) {
			logs.Warn("cann't get pd %s desired %d members", p.Db.GetName(), len(p.Members))
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		logs.Error("wait for pd %s available: %v", p.Db.GetName(), err)
	} else {
		logs.Debug("pd %s ok", p.Db.GetName())
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
