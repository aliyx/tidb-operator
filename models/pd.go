package models

import (
	"fmt"
	"time"

	"strings"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/pkg/k8sutil"
	"github.com/ffan/tidb-k8s/pkg/retryutil"
	"github.com/ghodss/yaml"
	"k8s.io/client-go/pkg/api/v1"
)

// Pd 元数据
type Pd struct {
	Spec Spec `json:"spec"`

	InnerAddresses []string `json:"innerAddresses,omitempty"`
	OuterAddresses []string `json:"outerAddresses,omitempty"`

	Member int `json:"member,omitempty"`
	// key is pod name
	Members map[string]Member `json:"members,omitempty"`

	Db *Tidb `json:"-"`
}

// Member describe a pd or tikv pod
type Member struct {
	ID    int `json:"id,omitempty"`
	State int `json:"state,omitempty"`
}

func (p *Pd) beforeSave() error {
	if err := p.Spec.validate(); err != nil {
		return err
	}
	md, _ := GetMetadata()
	max := md.Units.Pd.Max
	if p.Spec.Replicas < 3 || p.Spec.Replicas > max || p.Spec.Replicas%2 == 0 {
		return fmt.Errorf("replicas must be an odd number >= 3 and <= %d", max)
	}

	// set volume

	md, err := GetMetadata()
	if err != nil {
		return err
	}
	p.Spec.Volume = strings.Trim(md.K8s.Volume, " ")
	if len(p.Spec.Volume) == 0 {
		p.Spec.Volume = "emptyDir: {}"
	} else {
		p.Spec.Volume = fmt.Sprintf("hostPath: {path: %s}", p.Spec.Volume)
	}
	return nil
}

// GetPd  return a Pd
func GetPd(cell string) (*Pd, error) {
	db, err := GetTidb(cell)
	if err != nil {
		return nil, err
	}
	pd := db.Pd
	pd.Db = db
	return pd, nil
}

func (p *Pd) uninstall() (err error) {
	e := NewEvent(p.Db.Cell, "pd", "uninstall")
	defer func() {
		ph := pdStoped
		if err != nil {
			ph = pdStopFailed
		}
		p.Member = 0
		p.InnerAddresses = nil
		p.OuterAddresses = nil
		p.Db.Status.Phase = ph
		err = p.Db.update()
		if err != nil {
			logs.Error("uninstall pd %s: %v", p.Db.Cell, err)
		}
		e.Trace(err, "Uninstall pd pods")
	}()
	if err = k8sutil.DeletePodsBy(p.Db.Cell, "pd"); err != nil {
		return err
	}
	if err = k8sutil.DelSrvs(fmt.Sprintf("pd-%s", p.Db.Cell), fmt.Sprintf("pd-%s-srv", p.Db.Cell)); err != nil {
		return err
	}
	return err
}

func (p *Pd) install() (err error) {
	e := NewEvent(p.Db.Cell, "pd", "install")
	rollout(p.Db.Cell, pdPending)
	defer func() {
		ph := pdStarted
		if err != nil {
			ph = pdStartFailed
		}
		p.Db.Status.Phase = ph
		p.Db.update()
		e.Trace(err, fmt.Sprintf("Install pd services and pods with replicas desire: %d, running: %d on k8s", p.Spec.Replicas, p.Member))
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
	logs.Info("waiting for run pd %s ok...", p.Db.Cell)
	interval := 3 * time.Second
	err = retryutil.Retry(interval, int(waitTidbComponentAvailableTimeout/(interval)), func() (bool, error) {
		if _, err = pdLeaderGet(p.OuterAddresses[0]); err != nil {
			logs.Warn("get pd leader: %v", err)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		logs.Error("wait pd %s available: %v", p.Db.Cell, err)
	} else {
		logs.Info("pd %s ok", p.Db.Cell)
	}
	return err
}

func (p *Pd) toJSONTemplate(temp string) ([]byte, error) {
	r := strings.NewReplacer(
		"{{namespace}}", getNamespace(),
		"{{cell}}", p.Db.Cell,
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

func (p *Pd) isNil() bool {
	return p.Spec.Replicas < 1
}
