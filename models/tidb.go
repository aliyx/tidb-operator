package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/models/utils"

	tsql "github.com/ffan/tidb-k8s/mysql"

	"errors"
	"strconv"
	"sync"
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

	migrating          = "Migrating"
	migStartMigrateErr = "StartMigrationTaskError"

	scaling      = "Scaling"
	tikvScaleErr = "TikvScaleError"
	tidbScaleErr = "TikvScaleError"
)

var (
	tidbS Storage

	errCellIsNil = errors.New("cell is nil")
	// ErrRepop is returned by functions to specify the operation is executing.
	ErrRepop = errors.New("the previous operation is being executed")

	smu sync.Mutex
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

	Cell     string `json:"cell"`
	Schema   string `json:"schema"`
	User     string `json:"user"`
	Password string `json:"password"`

	Pd   *Pd   `json:"pd"`
	Tikv *Tikv `json:"tikv"`

	Status       TidbStatus `json:"status"`
	TimeCreate   time.Time  `json:"timecreate,omitempty"`
	MigrateState string     `json:"transfer,omitempty"`
	ScaleState   string     `json:"scale,omitempty"`
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
		return errCellIsNil
	}
	db.Cell = strings.Trim(db.Cell, " ")
	if err := db.Pd.beforeSave(); err != nil {
		return err
	}
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
	if err := db.K8sInfo.validate(); err != nil {
		return err
	}
	if old, _ := GetTidb(db.Cell); old != nil {
		return fmt.Errorf(`tidb "%s" has created`, old.Cell)
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

func rollout(cell string, s TidbStatus) error {
	db, err := GetTidb(cell)
	if err != nil {
		return err
	}
	db.Status = s
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
	if db.Pd != nil {
		db.Pd.Db = db
	}
	if db.Tikv != nil {
		db.Tikv.Db = db
	}
	return db, nil
}

func (db *Tidb) run() (err error) {
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
		db.Status = st
		db.Update()
		e.Trace(err, fmt.Sprintf("Start tidb replicationcontrollers with %d replicas on k8s", db.Replicas))
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
		"{{registry}}", dockerRegistry, "{{cell}}", db.Cell)
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

type clear func()

// Delete tidb from k8s
func (db *Tidb) Delete(callbacks ...clear) (err error) {
	if len(db.Cell) < 1 {
		return nil
	}
	if err = Stop(db.Cell, nil); err != nil {
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
	return nil
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
	td, err := GetTidb(cell)
	if err != nil {
		return err
	}
	if !td.Ok {
		return fmt.Errorf("tidbs not started")
	}
	if replicas == td.Replicas {
		return nil
	}
	td.Update()
	go func() {
		// Waiting for the end of the previous scale
		for i := 0; i < 60; i++ {
			td, err = GetTidb(cell)
			if err != nil || td.Status != TidbInited {
				logs.Error("tidb not started: %v", err)
				return
			}
			if td.ScaleState != scaling {
				td.ScaleState = scaling
				td.Update()
				break
			}
			time.Sleep(time.Second)
		}
		e := NewEvent(cell, "tidb", "scale")
		defer func(r int) {
			st := ""
			if err != nil {
				st = tidbScaleErr
			}
			td.ScaleState = st
			td.Update()
			e.Trace(err, fmt.Sprintf(`Scale tidb "%s" from %d to %d`, cell, r, replicas))
		}(td.Replicas)
		md, _ := GetMetadata()
		if replicas > md.Units.Tidb.Max {
			err = fmt.Errorf("the replicas of tidb exceeds max %d", md.Units.Tidb.Max)
			return
		}
		if replicas > td.Replicas*3 || td.Replicas > replicas*3 {
			err = fmt.Errorf("each scale can not more or less then 2 times")
			return
		}
		logs.Info(`Scale "tidb-%s" from %d to %d`, cell, td.Replicas, replicas)
		td.Replicas = replicas
		if err = td.validate(); err != nil {
			return
		}
		td.Update()
		if err = scaleReplicationcontroller(fmt.Sprintf("tidb-%s", cell), replicas); err != nil {
			return
		}
		if err = waitComponentRuning(startTidbTimeout, cell, "tidb"); err != nil {
			return
		}
	}()
	return nil
}

func (db *Tidb) isNil() bool {
	return db.Replicas < 1
}

func (db *Tidb) isOk() bool {
	return db.Status == TidbInited
}

func (db *Tidb) initSchema() (err error) {
	e := NewEvent(db.Cell, "tidb", "init")
	defer func() {
		e.Trace(err, "Init tidb privileges")
	}()
	if db.Status < TidbStarted || db.Status > TidbInited {
		return fmt.Errorf(`tidb "%s" no started`, db.Cell)
	}
	my := tsql.NewMysql(db.Schema, db.Nets[0].IP, db.Nets[0].Port, db.User, db.Password)
	if err = my.Init(); err != nil {
		rollout(db.Cell, TidbInitFailed)
		return err
	}
	rollout(db.Cell, TidbInited)
	return nil
}

// Start tidb server
func Start(cell string) (err error) {
	if started(cell) {
		return ErrRepop
	}
	var db *Tidb
	if db, err = GetTidb(cell); err != nil {
		logs.Error("Get tidb %s err: %v", cell, err)
		return err
	}
	go func() {
		e := NewEvent(cell, "tidb", "start")
		defer func() {
			e.Trace(err, "Start deploying tidb clusters on kubernetes")
		}()
		rollout(cell, PdPending)
		if err = db.Pd.run(); err != nil {
			logs.Error("Run pd %s on k8s err: %v", cell, err)
			return
		}
		rollout(cell, TikvPending)
		if err = db.Tikv.run(); err != nil {
			logs.Error("Run tikv %s on k8s err: %v", cell, err)
			return
		}
		rollout(cell, TidbPending)
		if err = db.run(); err != nil {
			logs.Error("Run tidb %s on k8s err: %v", cell, err)
			return
		}
		if err = db.initSchema(); err != nil {
			logs.Error("Init tidb %s privileges err: %v", cell, err)
			return
		}
	}()
	return nil
}

// Stop tidb server
func Stop(cell string, ch chan int) (err error) {
	if !started(cell) {
		return err
	}
	var db *Tidb
	if db, err = GetTidb(cell); err != nil {
		logs.Error("Get tidb %s err: %v", cell, err)
		return err
	}
	e := NewEvent(cell, "tidb", "stop")
	defer func() {
		if err != nil {
			e.Trace(err, fmt.Sprintf("Delete tidb pods on k8s"))
		}
	}()
	if err = stopMigrateTask(cell); err != nil {
		return err
	}
	if err = db.stop(); err != nil {
		return err
	}
	if err = db.Tikv.stop(); err != nil {
		return err
	}
	if err = db.Pd.stop(); err != nil {
		return err
	}
	// waitring for all pod deleted
	go func() {
		defer func() {
			if ch != nil {
				ch <- 0
			}
		}()
		for i := 0; i < 60; i++ {
			if started(cell) {
				logs.Warn(`tidb "%s" has not been cleared yet`, cell)
				time.Sleep(time.Second)
			} else {
				rollout(cell, Undefined)
				break
			}
		}
		var serr error
		if started(cell) {
			rollout(cell, TidbStopFailed)
			serr = errors.New("async delete pods timeout")
		}
		e.Trace(serr, fmt.Sprintf("Stop tidb pods on k8s"))
	}()
	return err
}

// Restart first stop tidb, second start tidb
func Restart(cell string) (err error) {
	go func() {
		td, _ := GetTidb(cell)
		e := NewEvent(cell, "tidb", "restart")
		defer func() {
			e.Trace(err, fmt.Sprintf("Restart tidb[status=%d]", td.Status))
		}()
		ch := make(chan int, 1)
		if err = Stop(cell, ch); err != nil {
			logs.Error("Delete tidb %s pods on k8s error: %v", cell, err)
			return
		}
		// waiting for all pod deleted
		select {
		case <-ch:
		}
		if err = Start(cell); err != nil {
			logs.Error("Create tidb %s pods on k8s error: %v", cell, err)
			return
		}
	}()
	return err
}

// Migrate the mysql data to the current tidb
func (db *Tidb) Migrate(src tsql.Mysql, notify string, sync bool) error {
	if !db.isOk() {
		return fmt.Errorf("tidb is not available")
	}
	// if db.MigrateState != "" {
	// 	return errors.New("can not migrate multiple times")
	// }
	if len(src.IP) < 1 || src.Port < 1 || len(src.User) < 1 || len(src.Password) < 1 || len(src.Database) < 1 {
		return fmt.Errorf("invalid database %+v", src)
	}
	if db.Schema != src.Database {
		return fmt.Errorf("both schemas must be the same")
	}
	var net Net
	for _, n := range db.Nets {
		if n.Name == portMysql {
			net = n
			break
		}
	}
	my := &tsql.Mydumper{
		Src:  src,
		Dest: *tsql.NewMysql(db.Schema, net.IP, net.Port, db.User, db.Password),

		IncrementalSync: sync,
		NotifyAPI:       notify,
	}
	if err := my.Check(); err != nil {
		return fmt.Errorf(`schema "%s" does not support migration error: %v`, db.Cell, err)
	}
	db.MigrateState = migrating
	if err := db.Update(); err != nil {
		return err
	}
	return db.startMigrateTask(my)
}

// UpdateMigrateStat update tidb migrate stat
func (db *Tidb) UpdateMigrateStat(s, desc string) (err error) {
	e := NewEvent(db.Cell, "Tidb", "migration")
	db.MigrateState = s
	if err := db.Update(); err != nil {
		return err
	}
	switch s {
	case "Dumping":
		e.Trace(err, "Dumping mysql data to local")
	case "DumpError":
		e.Trace(fmt.Errorf("Unknow"), "Dumped mysql data to local error")
	case "Loading":
		e.Trace(err, "Loading local data to tidb")
	case "LoadError":
		e.Trace(fmt.Errorf("Unknow"), "Loaded local data to tidb error")
	case "Finished":
		err = stopMigrateTask(db.Cell)
		e.Trace(err, "End migration task")
	case "Syncing":
		e.Trace(err, "Syncing mysql data to tidb")
	}
	return nil
}

func (db *Tidb) startMigrateTask(my *tsql.Mydumper) (err error) {
	sync := ""
	if my.IncrementalSync {
		sync = "sync"
	}
	r := strings.NewReplacer(
		"{{namespace}}", getNamespace(),
		"{{cell}}", db.Cell,
		"{{image}}", fmt.Sprintf("%s/migration:latest", dockerRegistry),
		"{{sh}}", my.Src.IP, "{{sP}}", fmt.Sprintf("%v", my.Src.Port),
		"{{su}}", my.Src.User, "{{sp}}", my.Src.Password,
		"{{db}}", my.Src.Database,
		"{{dh}}", my.Dest.IP, "{{dP}}", fmt.Sprintf("%v", my.Dest.Port),
		"{{duser}}", my.Dest.User, "{{dp}}", my.Dest.Password,
		"{{sync}}", sync,
		"{{api}}", my.NotifyAPI)
	s := r.Replace(k8sMigrate)
	go func() {
		e := NewEvent(db.Cell, "Tidb", "migration")
		defer func() {
			e.Trace(err, "Start migration task on k8s")
		}()
		if err = createPod(s); err != nil {
			return
		}
		if err = waitComponentRuning(startTidbTimeout, db.Cell, "migration"); err != nil {
			db.MigrateState = migStartMigrateErr
			err = db.Update()
		}
	}()
	return nil
}

func stopMigrateTask(cell string) error {
	return delPodsBy(cell, "migration")
}
