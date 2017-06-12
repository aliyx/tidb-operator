package models

import (
	"fmt"
	"strconv"

	"strings"

	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/pkg/k8sutil"
	"github.com/ffan/tidb-k8s/pkg/retryutil"
)

// Pd 元数据
type Pd struct {
	k8sutil.K8sInfo
	Volume string `json:"tidbdata_volume"`

	Db *Tidb `json:"-"`

	Member int `json:"member"`
	// key is pod name
	Members map[string]Member `json:"members,omitempty"`
}

// Member describe a pd or tikv pod
type Member struct {
	ID      int    `json:"id,omitempty"`
	Address string `json:"address,omitempty"`
	State   int    `json:"state,omitempty"`
}

// NewPd return a Pd instance
func NewPd() *Pd {
	return &Pd{}
}

func (p *Pd) beforeSave() error {
	if err := p.K8sInfo.Validate(); err != nil {
		return err
	}
	md, _ := GetMetadata()
	max := md.Units.Pd.Max
	if p.Replicas < 3 || p.Replicas > max || p.Replicas%2 == 0 {
		return fmt.Errorf("replicas must be an odd number >= 3 and <= %d", max)
	}

	// set volume

	md, err := GetMetadata()
	if err != nil {
		return err
	}
	p.Volume = strings.Trim(md.K8s.Volume, " ")
	if len(p.Volume) == 0 {
		p.Volume = "emptyDir: {}"
	} else {
		p.Volume = fmt.Sprintf("hostPath: {path: %s}", p.Volume)
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
		st := PdStoped
		if err != nil {
			st = PdStopFailed
		}
		p.Nets = nil
		p.Ok = false
		p.Db.Status = st
		p.Db.Update()
		e.Trace(err, "Uninstall pd pods")
	}()
	if err = k8sutil.DeletePodsByCell(p.Db.Cell); err != nil {
		return err
	}
	if err = k8sutil.DelSrvs(fmt.Sprintf("pd-%s", p.Db.Cell), p.Db.Cell); err != nil {
		return err
	}
	return err
}

func (p *Pd) install() (err error) {
	e := NewEvent(p.Db.Cell, "pd", "install")
	defer func() {
		st := PdStarted
		if err != nil {
			st = PdStartFailed
		} else {
			p.Ok = true
		}
		p.Db.Status = st
		p.Db.Update()
		e.Trace(err, fmt.Sprintf("Install pd pods with %d replicas on k8s", p.Replicas))
	}()
	if err = k8sutil.CreateService(p.toTemplate(pdServiceYaml)); err != nil {
		return err
	}
	if err = k8sutil.CreateService(p.toTemplate(pdHeadlessServiceYaml)); err != nil {
		return err
	}
	for i := 0; i < p.Replicas; i++ {
		p.Member++
		if err = k8sutil.CreateAndWaitPod(p.toTemplate(pdRcYaml)); err != nil {
			return err
		}
	}
	// Waiting for pds finished leader election
	if err = p.waitForComplete(startTidbTimeout); err != nil {
		return err
	}
	return err
}

func (p *Pd) waitForComplete(wait time.Duration) error {
	if err := k8sutil.WaitComponentRuning(wait, p.Db.Cell, "pd"); err != nil {
		return err
	}
	name := fmt.Sprintf("pd-%s", p.Db.Cell)
	cip, err := k8sutil.GetServiceProperties(name, `{{.spec.clusterIP}}:{{index (index .spec.ports 0) "nodePort"}}`)
	if err != nil || len(cip) == 0 {
		return fmt.Errorf("cannt get %s cluster ip, error: %v", name, err)
	}
	h := strings.Split(cip, ":")
	if len(h) != 2 {
		return fmt.Errorf("cannt get external cluster ip and port")
	}
	cip = h[0]
	nodePort, _ := strconv.Atoi(h[1])
	ps := getProxys()
	p.Nets = append(p.Nets, k8sutil.Net{portEtcd, cip, 2379})
	p.Nets = append(p.Nets, k8sutil.Net{portEtcdStatus, ps[0], nodePort})
	logs.Debug("Pd cluster host: %s external host: %s", p.Nets[0].String(), p.Nets[1].String())
	// must run on docker or k8s master node
	// 本地测试时注释掉
	if err := retryutil.RetryIfErr(wait, func() error {
		_, err = pdLeaderGet(p.Nets[0].String())
		return err
	}); err != nil {
		return fmt.Errorf(`waiting for service "%s" started timout`, name)
	}
	return nil
}

func (p *Pd) toTemplate(t string) string {
	r := strings.NewReplacer(
		"{{namespace}}", getNamespace(),
		"{{cell}}", p.Db.Cell,
		"{{id}}", fmt.Sprintf("%d", p.Member),
		"{{replicas}}", fmt.Sprintf("%d", p.Replicas),
		"{{cpu}}", fmt.Sprintf("%v", p.CPU),
		"{{mem}}", fmt.Sprintf("%v", p.Mem),
		"{{version}}", p.Version,
		"{{tidbdata_volume}}", p.Volume,
		"{{registry}}", imageRegistry,
	)
	s := r.Replace(t)
	return s
}

func (p *Pd) isNil() bool {
	return p.Replicas < 1
}
