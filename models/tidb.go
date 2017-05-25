package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/models/utils"

	"errors"
	"strconv"
)

// TidbStatus 描述tidb的创建/扩容/删除过程中每个节点的状态
type TidbStatus int

const (
	// Undefined 未开始创建
	Undefined TidbStatus = iota
	// PdPending pd待处理
	PdPending
	// PdStartFailed pd服务启动失败
	PdStartFailed
	// PdStarted pd模块服务可用
	PdStarted
	// TikvPending tikv待处理
	TikvPending
	// TikvStartFailed tikv服务启动失败
	TikvStartFailed
	// TikvStarted tikv服务可用
	TikvStarted
	// TidbPending tidb待处理
	TidbPending
	// TidbStartFailed tidb服务启动失败
	TidbStartFailed
	// TidbStarted tidb服务可用
	TidbStarted
	// TidbInitFailed 初始化失败
	TidbInitFailed
	// TidbInited 初始化完成，可对外提供服务
	TidbInited
	// TidbStopFailed fail to stop tidb
	TidbStopFailed
	// TidbStoped tidb服务已停止
	TidbStoped
	// TikvStopFailed fail to stop tikv
	TikvStopFailed
	// TikvStoped tikv服务已停止
	TikvStoped
	// PdStopFailed fail to stop tikv
	PdStopFailed
	// tidbClearing wait for k8s to close all pods
	tidbClearing
	// tidbDeleting wait for k8s to close all pods
	tidbDeleting
)

const (
	portMysql       = "mysql"
	portMysqlStatus = "mst"
	portEtcd        = "etcd"
	portEtcdStatus  = "est"
)

var (
	tidbS Storage

	k8sMu sync.Mutex
)

func tidbInit() {
	s, err := newStorage(tidbNamespace)
	if err != nil {
		panic(fmt.Errorf("Create storage tidb error: %v", err))
	}
	tidbS = s
}

// Tidb tidb数据库管理model
type Tidb struct {
	K8sInfo
	Schema   string `json:"schema"`
	User     string `json:"user"`
	Password string `json:"password"`

	Pd   *Pd   `json:"pd"`
	Tikv *Tikv `json:"tikv"`

	Status     TidbStatus `json:"status"`
	TimeCreate time.Time  `json:"timecreate,omitempty"`
	Transfer   string     `json:"transfer,omitempty"`
}

// NewTidb create a tidb instance
func NewTidb(cell ...string) *Tidb {
	td := Tidb{}
	if len(cell) > 0 {
		td.Cell = cell[0]
	}
	return &td
}

// Save tidb/tikv/pd info
func (db *Tidb) Save() error {
	if db.Cell == "" {
		return errors.New("cell is nil")
	}
	db.Pd.Cell = db.Cell
	if err := db.Pd.beforeSave(); err != nil {
		return err
	}
	db.Tikv.Cell = db.Cell
	if err := db.Tikv.beforeSave(); err != nil {
		return err
	}
	if err := db.beforeSave(); err != nil {
		return err
	}
	j, err := json.Marshal(db)
	if err != nil {
		return err
	}
	if err := tidbS.Create(db.Cell, j); err != nil {
		return err
	}
	return nil
}

// beforeSave 创建之前的检查工作
func (db *Tidb) beforeSave() error {
	if err := db.validate(); err != nil {
		return err
	}
	if old, _ := GetTidb(db.Cell); old != nil {
		return fmt.Errorf(`tidb "%s" has created`, old.Cell)
	}
	md, err := GetMetadata()
	if err != nil {
		return err
	}
	db.Registry = md.K8s.Registry
	return nil
}

func (db *Tidb) validate() error {
	if err := db.K8sInfo.validate(); err != nil {
		return err
	}
	return nil
}

// Update tidb metadata
func (db *Tidb) Update() error {
	if db.Cell == "" {
		return fmt.Errorf("cell is nil")
	}
	j, err := json.Marshal(db)
	if err != nil {
		return err
	}
	return tidbS.Update(db.Cell, j)
}

// tidbRollout 更新tidb的状态
func rollout(cell string, s TidbStatus) error {
	db, err := GetTidb(cell)
	if err != nil {
		return err
	}
	return db.Update()
}

func isPdOk(cell string) bool {
	if p, err := GetPd(cell); err != nil || p == nil || !p.Ok {
		return false
	}
	return true
}

// GetTidb get a Tidb instance
func GetTidb(cell string) (*Tidb, error) {
	bs, err := tidbS.Get(cell)
	if err != nil {
		return nil, err
	}
	db := NewTidb()
	if err := json.Unmarshal(bs, db); err != nil {
		return nil, err
	}
	return db, nil
}

// Run 启动tidb服务
func (db *Tidb) Run() (err error) {
	e := NewEvent(db.Cell, "tidb", "start")
	defer func() {
		st := TidbStarted
		if err != nil {
			st = TidbStartFailed
		} else {
			db.Ok = true
			if err = db.Update(); err != nil {
				st = TidbStartFailed
			}
		}
		e.Trace(err, fmt.Sprintf("Start tidb replicationcontrollers with %d replicas on k8s", db.Replicas))
		rollout(db.Cell, st)
	}()
	if err = createService(db.getK8sTemplate(k8sTidbService)); err != nil {
		return err
	}
	if err = createRc(db.getK8sTemplate(k8sTidbRc)); err != nil {
		return err
	}
	// wait tidb启动完成
	if err = db.waitForComplete(startTidbTimeout); err != nil {
		return err
	}
	return nil
}

func (db *Tidb) getK8sTemplate(t string) string {
	r := strings.NewReplacer(
		"{{version}}", db.Version,
		"{{cpu}}", fmt.Sprintf("%v", db.CPU), "{{mem}}", fmt.Sprintf("%v", db.Mem),
		"{{namespace}}", getNamespace(),
		"{{replicas}}", fmt.Sprintf("%v", db.Replicas),
		"{{registry}}", db.Registry, "{{cell}}", db.Cell)
	s := r.Replace(t)
	return s
}

func (db *Tidb) waitForComplete(wait time.Duration) error {
	if err := waitComponentRuning(wait, db.Cell, "tidb"); err != nil {
		return err
	}
	pts, err := getServiceProperties(
		fmt.Sprintf("tidb-%s", db.Cell),
		`{{index (index .spec.ports 0) "nodePort"}}:{{index (index .spec.ports 1) "nodePort"}}`)
	if err != nil || len(pts) == 0 {
		return fmt.Errorf(`cannt get tidb "%s" cluster ports: %v`, db.Cell, err)
	}
	pp := strings.Split(pts, ":")
	if len(pp) != 2 {
		return fmt.Errorf("cannt get external ports")
	}
	om, _ := strconv.Atoi(pp[0])
	os, _ := strconv.Atoi(pp[1])
	ps := getProxys()
	for _, p := range ps {
		db.Nets = append(db.Nets, Net{portMysql, p, om}, Net{portMysqlStatus, p, os})
	}
	// wait tidb status端口可访问
	if err := utils.RetryIfErr(wait, func() error {
		if _, err := utils.Get("http://"+db.Nets[1].String(), pdReqTimeout); err != utils.ErrNotFound {
			return err
		}
		return nil
	}); err != nil {
		return fmt.Errorf(`start up tidbs timout`)
	}
	logs.Debug("Tidb proxy: %v", db.Nets)
	return nil
}

// EraseTidb 清除tidb模块的数据
func EraseTidb(cell string) error {
	d, err := GetTidb(cell)
	if err != nil {
		return err
	}
	if d.isNil() {
		return nil
	}
	if err = d.stop(); err != nil {
		return err
	}
	d.clear()
	logs.Debug("%+v", d)
	if err = d.Update(); err != nil {
		return err
	}
	return nil
}

func (db *Tidb) stop() (err error) {
	e := NewEvent(db.Cell, "tidb", "stop")
	defer func() {
		st := TidbStoped
		if err != nil {
			st = TidbStopFailed
		} else {
		}
		db.Nets = nil
		db.Ok = false
		e.Trace(err, "Stop tidb replicationcontrollers")
		db.Update()
		rollout(db.Cell, st)
	}()
	if err = delRc(fmt.Sprintf("tidb-%s", db.Cell)); err != nil {
		return err
	}
	if err = delSrvs(fmt.Sprintf("tidb-%s", db.Cell)); err != nil {
		return err
	}
	return err
}

func (db *Tidb) clear() {
	cell := db.Cell
	db.K8sInfo = K8sInfo{}
	db.Cell = cell
	db.User = ""
	db.Password = ""
}

type clear func()

// Delete tidb from k8s
func (db *Tidb) Delete(callbacks ...clear) (err error) {
	if len(db.Cell) < 1 {
		return nil
	}
	if err = EraseTidb(db.Cell); err != nil && err != ErrNoNode {
		logs.Error("Erase tikv %s: %v", db.Cell, err)
		return err
	}
	if err = DeleteTikv(db.Cell); err != nil && err != ErrNoNode {
		logs.Error("Delete tikv %s: %v", db.Cell, err)
		return err
	}
	if err = DeletePd(db.Cell); err != nil && err != ErrNoNode {
		logs.Error("Delete pd %s: %v", db.Cell, err)
		return err
	}
	if err = delEventsBy(db.Cell); err != nil {
		logs.Error("Delete events: %v", err)
		return err
	}
	go func() {
		rollout(db.Cell, tidbDeleting)
		for {
			if !started(db.Cell) {
				if err := db.delete(); err != nil && err != ErrNoNode {
					logs.Error("delete tidb error: %v", err)
					return
				}
				if len(callbacks) > 0 {
					for _, call := range callbacks {
						call()
					}
				}
				return
			}
			time.Sleep(time.Second)
		}
	}()
	return
}

func started(cell string) bool {
	pods, err := listPodNames(cell, "")
	if err != nil {
		logs.Warn("Get %s pods error: %v", cell, err)
	}
	return len(pods) > 0
}

func (db *Tidb) delete() error {
	if err := tidbS.Delete(db.Cell); err != nil {
		return err
	}
	logs.Warn(`Tidb "%s" deleted`, db.Cell)
	return nil
}

// ScaleTidbs 扩容tidb模块
func ScaleTidbs(replicas int, cell string) error {
	k8sMu.Lock()
	defer k8sMu.Unlock()
	db, err := GetTidb(cell)
	if err != nil {
		return err
	}
	if !db.Ok {
		return fmt.Errorf("tidbs not started")
	}
	if replicas == db.Replicas {
		return nil
	}
	md, _ := GetMetadata()
	if replicas > md.Units.Tidb.Max {
		return fmt.Errorf("the replicas of tidb exceeds max %d", md.Units.Tidb.Max)
	}
	if replicas > db.Replicas*3 || db.Replicas > replicas*3 {
		return fmt.Errorf("each expansion can not more or less then 2 times")
	}
	e := NewEvent(cell, "tidb", "scale")
	defer func() {
		e.Trace(err, fmt.Sprintf(`Scale tidb "%s" from %d to %d`, cell, db.Replicas, replicas))
	}()
	logs.Info(`Scale "tidb-%s" from %d to %d`, cell, db.Replicas, replicas)
	db.Replicas = replicas
	if err = db.validate(); err != nil {
		return err
	}
	db.Update()
	if err = scaleReplicationcontroller(fmt.Sprintf("tidb-%s", cell), replicas); err != nil {
		return nil
	}
	if err = waitComponentRuning(startTidbTimeout, cell, "tidb"); err != nil {
		return err
	}
	return nil
}

func (db *Tidb) isNil() bool {
	return db.Replicas < 1
}

func (db *Tidb) isOk() bool {
	return db.Status == TidbInited
}
