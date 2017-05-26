package models

import (
	"fmt"
	"strconv"

	"strings"

	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/models/utils"
)

// Pd 元数据
type Pd struct {
	K8sInfo

	Db *Tidb `json:"-"`
}

// NewPd return a Pd instance
func NewPd() *Pd {
	return &Pd{}
}

func (p *Pd) beforeSave() error {
	if err := p.K8sInfo.validate(); err != nil {
		return err
	}
	md, _ := GetMetadata()
	max := md.Units.Pd.Max
	if p.Replicas < 3 || p.Replicas > max || p.Replicas%2 == 0 {
		return fmt.Errorf("replicas must be an odd number >= 3 and <= %d", max)
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

func (p *Pd) update() error {
	return p.Db.Update()
}

func (p *Pd) stop() (err error) {
	e := NewEvent(p.Db.Cell, "Pd", "stop")
	defer func() {
		st := tidbClearing
		if err != nil {
			st = PdStopFailed
		}
		rollout(p.Db.Cell, st)
		e.Trace(err, "Stop pd replicationcontrollers")
	}()
	if err = delRc(fmt.Sprintf("pd-%s", p.Db.Cell)); err != nil {
		return err
	}
	if err = delSrvs(fmt.Sprintf("pd-%s", p.Db.Cell), fmt.Sprintf("pd-%s-srv", p.Db.Cell)); err != nil {
		return err
	}
	return err
}

func (p *Pd) run() (err error) {
	e := NewEvent(p.Db.Cell, "Pd", "start")
	defer func() {
		st := PdStarted
		if err != nil {
			st = PdStartFailed
		} else {
			p.Ok = true
		}
		p.Db.Status = st
		p.Db.Update()
		e.Trace(err, fmt.Sprintf("Start pd replicationcontrollers with %d replicas on k8s", p.Replicas))
	}()
	if err = createService(p.getK8sTemplate(k8sPdService)); err != nil {
		return err
	}
	if err = createService(p.getK8sTemplate(k8sPdHeadlessService)); err != nil {
		return err
	}
	if err = createRc(p.getK8sTemplate(k8sPdRc)); err != nil {
		return err
	}
	// waiting for pds完成leader选举
	if err = p.waitForComplete(startTidbTimeout); err != nil {
		return err
	}
	return err
}

func (p *Pd) waitForComplete(wait time.Duration) error {
	if err := waitComponentRuning(wait, p.Db.Cell, "pd"); err != nil {
		return err
	}
	name := fmt.Sprintf("pd-%s", p.Db.Cell)
	cip, err := getServiceProperties(name, `{{.spec.clusterIP}}:{{index (index .spec.ports 0) "nodePort"}}`)
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
	p.Nets = append(p.Nets, Net{portEtcd, cip, 2379})
	p.Nets = append(p.Nets, Net{portEtcdStatus, ps[0], nodePort})
	logs.Debug("Pd cluster host: %s external host: %s", p.Nets[0].String(), p.Nets[1].String())
	// must run on docker or k8s master node
	// 本地测试时注释掉
	if err := utils.RetryIfErr(wait, func() error {
		_, err = pdLeaderGet(p.Nets[0].String())
		return err
	}); err != nil {
		return fmt.Errorf(`waiting for service "%s" started timout`, name)
	}
	return nil
}

// genK8sTemplate 生成k8s pd template
func (p *Pd) getK8sTemplate(t string) string {
	r := strings.NewReplacer("{{version}}", p.Version,
		"{{cpu}}", fmt.Sprintf("%v", p.CPU),
		"{{mem}}", fmt.Sprintf("%v", p.Mem),
		"{{replicas}}", fmt.Sprintf("%v", p.Replicas),
		"{{registry}}", dockerRegistry,
		"{{cell}}", p.Db.Cell)
	s := r.Replace(t)
	return s
}

func (p *Pd) isNil() bool {
	return p.Replicas < 1
}
