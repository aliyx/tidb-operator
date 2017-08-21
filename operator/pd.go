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
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

func (p *Pd) upgrade() error {
	if len(p.Members) < 1 {
		return nil
	}
	if p.Db.Status.Phase < PhasePdStarted {
		return fmt.Errorf("the tidb %q pd unavailable", p.Db.GetName())
	}

	var (
		err      error
		upgraded bool
		count    int
	)

	e := NewEvent(p.Db.GetName(), "tidb/pd", "upgrate")
	defer func() {
		// have upgrade
		if count > 0 || err != nil {
			e.Trace(err, fmt.Sprintf("Upgrate pd to version: %s", p.Version))
		}
	}()

	for _, mb := range p.Members {
		if mb.State == PodFailed {
			continue
		}
		upgraded, err = upgradeOne(mb.Name, p.getImage(), p.Version)
		if err != nil {
			mb.State = PodFailed
			return err
		}
		if upgraded {
			count++
			// wait election and sync data
			time.Sleep(pdUpgradeInterval)
			if err = p.waitForOk(); err != nil {
				mb.State = PodFailed
				return err
			}
		}
	}
	return nil
}

// https://coreos.com/etcd/docs/latest/op-guide/runtime-configuration.html
// https://coreos.com/etcd/docs/latest/op-guide/failures.html
// The process is as follows:
// 1.Replace a failed pd, join the exist cluster if normal; otherwise reboot a new same pd with old name.
// 2.Delete uncontrolled member to <db.Tikv.Members> as the center.
func (db *Db) reconcilePds() error {
	var (
		err     error
		p       = db.Pd
		changed = 0
		pods    []apiv1.Pod
	)

	e := db.Event(eventPd, "reconcile")
	defer func() {
		parseError(db, err)
		if changed > 0 || err != nil {
			if err != nil {
				logs.Error("reconcile pd %q cluster error: %v", db.GetName(), err)
			}
			e.Trace(err, "Reconcile pd cluster")
		}
	}()

	pods, err = k8sutil.GetPods(db.GetName(), "pd")
	if err != nil {
		return err
	}

	// mark not running pod
	for _, mb := range p.Members {
		st := PodFailed
		for _, pod := range pods {
			if pod.GetName() == mb.Name && k8sutil.IsPodOk(pod) {
				st = PodRunning
				break
			}
		}
		mb.State = st
	}

	// delete failed pod and create a new pod
	for i, mb := range p.Members {
		if mb.State == PodRunning {
			continue
		}
		changed++
		logs.Info("start deleting member %q, because it is not available", mb.Name)
		tries := 3
		for i := 0; i < tries; i++ {
			if err = pdutil.PdMemberDelete(p.OuterAddresses[0], mb.Name); err == nil {
				// rejoin a deleted pd
				p.join = true
				break
			}
			logs.Warn("retry delete member %q: %d times", mb.Name, i+1)
			// maybe is electing
			time.Sleep(time.Duration(i) * time.Second)
		}
		if err != nil {
			// maybe majority members of the cluster fail,
			// the etcd cluster fails and cannot accept more writes
			logs.Critical("failed to delete member, because pd %q cluster is unavailable", p.Db.GetName())
		}
		if err = k8sutil.DeletePod(mb.Name, terminationGracePeriodSeconds); err != nil {
			return err
		}
		p.Member = i + 1
		p.initialClusterState = "existing"
		if err = p.createPod(); err != nil {
			return err
		}
		mb.State = PodRunning
	}

	if changed > 0 {
		if err = p.waitForOk(); err != nil {
			return err
		}
	}

	// check pd cluster whether normal
	js, err := pdutil.RetryGetPdMembers(p.OuterAddresses[0])
	if err != nil {
		logs.Critical("pd %q cluster is unavailable", p.Db.GetName())
		// Perhaps because of pod missing can not be elected
		return err
	}

	// Remove uncontrolled member
	ret := gjson.Get(js, "members.#.name")
	if ret.Type == gjson.Null {
		logs.Critical("could not get pd %q members, maybe cluster is unavailable", p.Db.GetName())
		return ErrPdUnavailable
	}
	for _, r := range ret.Array() {
		have := false
		for _, mb := range p.Members {
			if r.String() == mb.Name {
				have = true
				break
			}
		}
		if !have {
			logs.Warn("delete member %s from pd cluster", r.String())
			if err = pdutil.PdMemberDelete(p.OuterAddresses[0], r.String()); err != nil {
				return err
			}
		}
	}

	// Add missing member
	for i, mb := range p.Members {
		have := false
		for _, r := range ret.Array() {
			if r.String() == mb.Name {
				have = true
				break
			}
		}
		if !have {
			changed++
			logs.Info("restart tikv %q, becase of that not in pd cluster", mb.Name)
			if err = k8sutil.DeletePod(mb.Name, terminationGracePeriodSeconds); err != nil {
				return err
			}
			p.Member = i + 1
			p.join = true
			p.initialClusterState = "existing"
			if err = p.createPod(); err != nil {
				return err
			}
			mb.State = PodRunning
		}
	}
	if changed > 0 {
		if err = p.waitForOk(); err != nil {
			return err
		}
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
	e := p.Db.Event(eventPd, "install")
	defer func() {
		ph := PhasePdStarted
		if err != nil {
			ph = PhasePdStartFailed
		}
		p.Db.Status.Phase = ph
		e.Trace(err,
			fmt.Sprintf("Install pd services and pods with replicas desire: %d, running: %d on k8s", p.Replicas, p.Member))
	}()

	// savepoint for page show
	p.Db.Status.Phase = PhasePdPending
	if err = p.Db.patch(nil); err != nil {
		return err
	}

	if err = p.createServices(); err != nil {
		return err
	}

	for i := 0; i < p.Replicas; i++ {
		p.Member++
		st := PodRunning
		if err = p.createPod(); err != nil {
			st = PodFailed
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

	// Waiting for all pds available
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

func (p *Pd) createService(temp string) (*apiv1.Service, error) {
	j, err := p.toJSONTemplate(temp)
	if err != nil {
		return nil, err
	}
	return k8sutil.CreateServiceByJSON(j)
}

func (p *Pd) createPod() error {
	var (
		err error
		b   []byte
	)
	if b, err = p.toJSONTemplate(pdPodYaml); err != nil {
		return err
	}
	if _, err = k8sutil.CreatePodByJSON(b, waitPodRuningTimeout, func(pod *apiv1.Pod) {
		k8sutil.SetTidbVersion(pod, p.Version)
	}); err != nil {
		return err
	}
	return nil
}

func (p *Pd) waitForOk() (err error) {
	logs.Debug("wait for pd %q running", p.Db.GetName())
	interval := 3 * time.Second
	err = retryutil.Retry(interval, int(waitTidbComponentAvailableTimeout/(interval)), func() (bool, error) {
		if _, err = pdutil.PdLeaderGet(p.OuterAddresses[0]); err != nil {
			logs.Warn("could not get pd %q leader: %v", p.Db.GetName(), err)
			return false, nil
		}
		js, err := pdutil.PdMembersGet(p.OuterAddresses[0])
		if err != nil {
			return false, err
		}
		ret := gjson.Get(js, "members.#.name")
		if ret.Type == gjson.Null {
			logs.Warn("could not get pd %s members", p.Db.GetName())
			return false, nil
		}
		if len(ret.Array()) != len(p.Members) {
			logs.Warn("could not get pd %q desired members count: %d, current: %d",
				p.Db.GetName(), len(p.Members), len(ret.Array()))
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		logs.Error("wait for pd %q available: %v", p.Db.GetName(), err)
	} else {
		logs.Debug("pd %q ok", p.Db.GetName())
	}
	return err
}

func (p *Pd) toJSONTemplate(temp string) ([]byte, error) {
	state := "new"
	cluster := "--initial-cluster=$urls"
	if p.initialClusterState != "" {
		state = p.initialClusterState
		p.initialClusterState = ""
	}
	if p.join {
		cluster = "--join=http://pd-" + p.Db.GetName() + ":2379"
		p.join = false
	}
	r := strings.NewReplacer(
		"{{namespace}}", getNamespace(),
		"{{cell}}", p.Db.GetName(),
		"{{id}}", fmt.Sprintf("%03d", p.Member),
		"{{replicas}}", fmt.Sprintf("%d", p.Spec.Replicas),
		"{{cpu}}", fmt.Sprintf("%v", p.Spec.CPU),
		"{{mem}}", fmt.Sprintf("%v", p.Spec.Mem),
		"{{version}}", p.Spec.Version,
		"{{registry}}", imageRegistry,
		"{{c-state}}", state,
		"{{c-urls}}", cluster,
	)
	str := r.Replace(temp)
	j, err := yaml.YAMLToJSON([]byte(str))
	if err != nil {
		return nil, err
	}
	return j, nil
}

func (p *Pd) getImage() string {
	return fmt.Sprintf("%s/pd:%s", imageRegistry, p.Version)
}
