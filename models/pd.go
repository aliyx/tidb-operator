package models

import (
	"fmt"
	"math"
	"strconv"

	"strings"

	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/models/utils"
)

// Pd 元数据
type Pd struct {
	K8sInfo

	Db Tidb `json:"-"`
}

// NewPd return a Pd instance
func NewPd() *Pd {
	return &Pd{}
}

func (p *Pd) beforeSave() error {
	if err := p.validate(); err != nil {
		return err
	}
	if old, _ := GetPd(p.Cell); old != nil {
		return fmt.Errorf(`pd "%s" has been created`, p.Cell)
	}
	md, err := GetMetadata()
	if err != nil {
		return err
	}
	p.Registry = md.K8s.Registry
	return nil
}

func (p *Pd) validate() error {
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
	if db.Pd == nil {
		return nil, ErrNoNode
	}
	pd := db.Pd
	pd.Db = *db
	return db.Pd, nil
}

// Update pd
func (p *Pd) Update() error {
	db, err := GetTidb(p.Cell)
	if err != nil {
		return err
	}
	db.Pd = p
	if err := db.Update(); err != nil {
		return err
	}
	return nil
}

// ScalePds 扩容pd模块,replicas必须是大于3的奇数
func ScalePds(replicas int, cell string) error {
	k8sMu.Lock()
	defer k8sMu.Unlock()
	pd, err := GetPd(cell)
	if err != nil || pd == nil || !pd.Ok {
		return fmt.Errorf("module pd not started: %v", err)
	}
	if replicas == pd.Replicas {
		return nil
	}
	if replicas > pd.Replicas*3 || pd.Replicas > replicas*3 {
		return fmt.Errorf("each expansion can not more or less then 2 times")
	}

	e := NewEvent(cell, "pd", "scale")
	defer func() {
		e.Trace(err, fmt.Sprintf(`Scale pd "%s" from %d to %d`, cell, pd.Replicas, replicas))
	}()
	pd.Replicas = replicas
	if err = pd.validate(); err != nil {
		return err
	}
	logs.Info(`Scale "pd-%s" from %d to %d`, cell, pd.Replicas, replicas)
	pd.Update()
	if err = scaleReplicationcontroller(fmt.Sprintf("pd-%s", cell), replicas); err != nil {
		return nil
	}
	cip := pd.Nets[0].String()
	if err = utils.RetryIfErr(startTidbTimeout, func() error {
		mems, err := pdMembersGetName(cip)
		if err != nil {
			return err
		}
		if len(mems) != replicas {
			return fmt.Errorf("part of the pod d'not start up, pods: %d members: %d", replicas, len(mems))
		}
		return nil
	}); err != nil {
		return fmt.Errorf(`waiting for scale "pd-%s" timeout`, cell)
	}
	return nil
}

// DeletePd 删除pd服务
func DeletePd(cell string) (err error) {
	if kv, _ := GetTikv(cell); kv != nil {
		return fmt.Errorf(`please delete tikv "%s" first`, cell)
	}
	var p *Pd
	p, err = GetPd(cell)
	if err != nil {
		return err
	}
	if err = p.stop(); err != nil {
		return err
	}
	if err := p.delete(); err != nil {
		return err
	}
	logs.Info(`Pd "%s" deleted`, p.Cell)
	return nil
}

func (p *Pd) delete() error {
	db, err := GetTidb(p.Cell)
	if err != nil {
		return err
	}
	db.Pd = nil
	return db.Update()
}

func (p *Pd) stop() (err error) {
	e := NewEvent(p.Cell, "Pd", "stop")
	defer func() {
		e.Trace(err, "Stop pd replicationcontrollers")
	}()
	defer func() {
		st := tidbClearing
		if err != nil {
			st = PdStopFailed
		}
		p.Ok = false
		p.Nets = nil
		p.Update()
		rollout(p.Cell, st)
	}()
	if err = delRc(fmt.Sprintf("pd-%s", p.Cell)); err != nil {
		return err
	}
	if err = delSrvs(fmt.Sprintf("pd-%s", p.Cell), fmt.Sprintf("pd-%s-srv", p.Cell)); err != nil {
		return err
	}
	return err
}

// GetPdReplicas 通过tikv和tidb的replicas计算需要pd的replicas
func GetPdReplicas(kv, db int) (int, error) {
	if db < 1 {
		db = 1
	}
	if kv < 3 {
		kv = 3
	}
	md, err := GetMetadata()
	if err != nil {
		return 0, err
	}
	pds := kv / 3
	if pds < 3 {
		pds = 3
	}
	if pds > md.Units.Pd.Max {
		pds = md.Units.Pd.Max
	}
	if pds%2 == 0 {
		pds = pds + 1
	}
	return pds, nil
}

// Run 启动pd集群
func (p *Pd) Run() (err error) {
	e := NewEvent(p.Cell, "Pd", "start")
	defer func() {
		st := PdStarted
		if err != nil {
			st = PdStartFailed
		} else {
			p.Ok = true
		}
		e.Trace(err, fmt.Sprintf("Start pd replicationcontrollers with %d replicas on k8s", p.Replicas))
		p.Update()
		rollout(p.Cell, st)
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
	if err := waitComponentRuning(wait, p.Cell, "pd"); err != nil {
		return err
	}
	name := fmt.Sprintf("pd-%s", p.Cell)
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
		"{{registry}}", p.Registry,
		"{{cell}}", p.Cell)
	s := r.Replace(t)
	return s
}

// dynamicScalePds 根据tikv和tidb的数量动态调整pd的数量
func dynamicScalePds(cell string) error {
	pd, err := GetPd(cell)
	if err != nil {
		return err
	}
	db, err := GetTidb(cell)
	if err != nil {
		return err
	}
	kv, err := GetTikv(cell)
	if err != nil {
		return err
	}
	// scale tikv之后，看是否需要scale pd
	pc, err := GetPdReplicas(kv.Replicas, db.Replicas)
	if err != nil {
		return err
	}

	if math.Abs(float64(pc)-float64(pd.Replicas))/float64(pd.Replicas) <= pdScaleFactor {
		return nil
	}
	pd.Replicas = pc
	if err := pd.Update(); err != nil {
		return err
	}
	logs.Warn("scale pd from %d to %d", pd.Replicas, pc)
	if err = ScalePds(pc, cell); err != nil {
		return fmt.Errorf("scale pd error: %v", err)
	}
	return nil
}

func (p *Pd) isNil() bool {
	return p.Replicas < 1
}
