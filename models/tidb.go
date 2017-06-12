package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/astaxie/beego/logs"

	"github.com/ffan/tidb-k8s/pkg/k8sutil"
	tsql "github.com/ffan/tidb-k8s/pkg/mysqlutil"
	"github.com/ffan/tidb-k8s/pkg/retryutil"

	"errors"
	"strconv"
	"sync"

	"github.com/ffan/tidb-k8s/pkg/httputil"
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
	// PdStoped  delete pd pods from k8s
	PdStoped
	// tidbDeleting wait for delete tidb
	tidbDeleting
)

const (
	portMysql       = "mysql"
	portMysqlStatus = "mst"
	portEtcd        = "etcd"
	portEtcdStatus  = "est"

	migrating          = "Migrating"
	migStartMigrateErr = "StartMigrationTaskError"

	defaultStopTidbTimeout = 60 // 60s

	scaling      = 1 << 8
	tikvScaleErr = 1
	tidbScaleErr = 1 << 1
)

var (
	tidbS Storage

	errCellIsNil = errors.New("cell is nil")
	// ErrRepeatOperation is returned by functions to specify the operation is executing.
	ErrRepeatOperation = errors.New("the previous operation is being executed")

	scaleMu sync.Mutex

	errInvalidReplica = errors.New("invalid replica")
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
	k8sutil.K8sInfo

	Cell     string `json:"cell"`
	Schema   string `json:"schema"`
	User     string `json:"user"`
	Password string `json:"password"`

	Pd   *Pd   `json:"pd"`
	Tikv *Tikv `json:"tikv"`

	Status       TidbStatus `json:"status"`
	TimeCreate   time.Time  `json:"timecreate,omitempty"`
	MigrateState string     `json:"transfer,omitempty"`
	ScaleState   int        `json:"scale,omitempty"`
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
	if err := db.K8sInfo.Validate(); err != nil {
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
	if err = k8sutil.CreateService(db.getK8sTemplate(tidbServiceYaml)); err != nil {
		return err
	}
	if err = k8sutil.CreateRc(db.getK8sTemplate(tidbRcYaml)); err != nil {
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
		"{{registry}}", imageRegistry, "{{cell}}", db.Cell)
	s := r.Replace(t)
	return s
}

func (db *Tidb) waitForComplete(wait time.Duration) error {
	if err := k8sutil.WaitComponentRuning(wait, db.Cell, "tidb"); err != nil {
		return err
	}
	pts, err := k8sutil.GetServiceProperties(
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
		db.Nets = append(db.Nets, k8sutil.Net{portMysql, p, om}, k8sutil.Net{portMysqlStatus, p, os})
	}
	// wait tidb status端口可访问
	if err := retryutil.RetryIfErr(wait, func() error {
		if _, err := httputil.Get("http://"+db.Nets[1].String(), pdReqTimeout); err != httputil.ErrNotFound {
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
		db.Status = st
		db.MigrateState = ""
		db.ScaleState = 0
		db.Update()
		e.Trace(err, "Stop tidb replicationcontrollers")
	}()
	if err = k8sutil.DelRc(fmt.Sprintf("tidb-%s", db.Cell)); err != nil {
		return err
	}
	if err = k8sutil.DelSrvs(fmt.Sprintf("tidb-%s", db.Cell)); err != nil {
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
	pods, err := k8sutil.ListPodNames(cell, "")
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

// Scale tikv and tidb
func Scale(cell string, kvReplica, dbReplica int) (err error) {
	scaleMu.Lock()
	defer scaleMu.Unlock()
	var db *Tidb
	if db, err = GetTidb(cell); err != nil {
		return err
	}
	if db.Status != TidbInited {
		return fmt.Errorf("tidb %s not inited", cell)
	}
	if db.ScaleState&scaling > 0 {
		return fmt.Errorf("tidb %s is scaling", cell)
	}
	db.ScaleState |= scaling
	db.Update()
	var wg sync.WaitGroup
	db.scaleTikvs(kvReplica, &wg)
	db.scaleTidbs(dbReplica, &wg)
	go func() {
		wg.Wait()
		db.ScaleState ^= scaling
		db.Update()
	}()
	return nil
}

func (db *Tidb) scaleTidbs(replica int, wg *sync.WaitGroup) {
	if replica < 1 {
		return
	}
	if replica == db.Replicas {
		return
	}
	wg.Add(1)
	go func() {
		scaleMu.Lock()
		defer func() {
			scaleMu.Unlock()
			wg.Done()
		}()
		var err error
		e := NewEvent(db.Cell, "tidb", "scale")
		defer func(r int) {
			if err != nil {
				db.ScaleState |= tidbScaleErr
			}
			db.Update()
			e.Trace(err, fmt.Sprintf(`Scale tidb "%s" replica: %d->%d`, db.Cell, r, replica))
		}(db.Replicas)
		md, _ := GetMetadata()
		if replica > md.Units.Tidb.Max {
			err = fmt.Errorf("the replicas of tidb exceeds max %d", md.Units.Tidb.Max)
			return
		}
		if replica > db.Replicas*3 || db.Replicas > replica*3 {
			err = fmt.Errorf("each scale can not more or less then 2 times")
			return
		}
		old := db.Replicas
		db.Replicas = replica
		if err = db.Validate(); err != nil {
			db.Replicas = old
			return
		}
		logs.Info(`Scale "tidb-%s" from %d to %d`, db.Cell, db.Replicas, replica)
		if err = k8sutil.ScaleReplicationcontroller(fmt.Sprintf("tidb-%s", db.Cell), replica); err != nil {
			return
		}
		if err = k8sutil.WaitComponentRuning(startTidbTimeout, db.Cell, "tidb"); err != nil {
			return
		}
	}()
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
		return ErrRepeatOperation
	}
	var db *Tidb
	if db, err = GetTidb(cell); err != nil {
		logs.Error("Get tidb %s err: %v", cell, err)
		return err
	}
	go func() {
		e := NewEvent(cell, "tidb", "start")
		defer func() {
			e.Trace(err, "Start deploying tidb cluster on kubernete")
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
		return err
	}
	e := NewEvent(cell, "tidb", "stop")
	defer func() {
		if err != nil {
			e.Trace(err, "Stop tidb pods on k8s")
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
	// waiting for all pods deleted from k8s
	go func() {
		defer func() {
			stoped := 1
			st := Undefined
			if started(cell) {
				st = TidbStopFailed
				stoped = 0
				err = errors.New("async delete pods timeout")
			}
			rollout(cell, st)
			if ch != nil {
				ch <- stoped
			}
			e.Trace(err, "Stop tidb pods on k8s")
		}()
		for i := 0; i < defaultStopTidbTimeout; i++ {
			if started(cell) {
				logs.Warn(`tidb "%s" has not been cleared yet`, cell)
				time.Sleep(time.Second)
			} else {
				break
			}
		}
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
		stoped := <-ch
		if stoped == 0 {
			logs.Error("stop tidb %s timeout", cell)
			return
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
	var net k8sutil.Net
	for _, n := range db.Nets {
		if n.Name == portMysql {
			net = n
			break
		}
	}
	my := &tsql.Migration{
		Src:  src,
		Dest: *tsql.NewMysql(db.Schema, net.IP, net.Port, db.User, db.Password),

		ToggleSync: sync,
		NotifyAPI:  notify,
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
	var e *Event
	db.MigrateState = s
	if err := db.Update(); err != nil {
		return err
	}
	logs.Info("Current tidb %s migrate status: %s", db.Cell, s)
	switch s {
	case "Dumping":
		e = NewEvent(db.Cell, "migration", "dump")
		e.Trace(nil, "Start Dumping mysql data to local")
	case "DumpError":
		e = NewEvent(db.Cell, "migration", "dump")
		e.Trace(fmt.Errorf("Unknow"), "Dumped mysql data to local error")
	case "Loading":
		e = NewEvent(db.Cell, "migration", "load")
		e.Trace(nil, "End dumped and start loading local to tidb")
	case "LoadError":
		e = NewEvent(db.Cell, "migration", "load")
		e.Trace(fmt.Errorf("Unknow"), "Loaded local data to tidb error")
	case "Finished":
		e = NewEvent(db.Cell, "tidb", "migration")
		err = stopMigrateTask(db.Cell)
		e.Trace(err, "End the full migration and delete migration docker on k8s")
	case "Syncing":
		e = NewEvent(db.Cell, "migration", "sync")
		e.Trace(nil, "Finished load and start incremental syncing mysql data to tidb")
	}
	return nil
}

func (db *Tidb) startMigrateTask(my *tsql.Migration) (err error) {
	sync := ""
	if my.ToggleSync {
		sync = "sync"
	}
	r := strings.NewReplacer(
		"{{namespace}}", getNamespace(),
		"{{cell}}", db.Cell,
		"{{image}}", fmt.Sprintf("%s/migration:latest", imageRegistry),
		"{{sh}}", my.Src.IP, "{{sP}}", fmt.Sprintf("%v", my.Src.Port),
		"{{su}}", my.Src.User, "{{sp}}", my.Src.Password,
		"{{db}}", my.Src.Database,
		"{{dh}}", my.Dest.IP, "{{dP}}", fmt.Sprintf("%v", my.Dest.Port),
		"{{duser}}", my.Dest.User, "{{dp}}", my.Dest.Password,
		"{{sync}}", sync,
		"{{api}}", my.NotifyAPI)
	s := r.Replace(mysqlMigraeYaml)
	go func() {
		e := NewEvent(db.Cell, "tidb", "migration")
		defer func() {
			e.Trace(err, "Startup migration docker on k8s")
		}()
		if err = k8sutil.CreatePod(s); err != nil {
			return
		}
		if err = k8sutil.WaitComponentRuning(startTidbTimeout, db.Cell, "migration"); err != nil {
			db.MigrateState = migStartMigrateErr
			err = db.Update()
		}
	}()
	return nil
}

func stopMigrateTask(cell string) error {
	return k8sutil.DelPodsBy(cell, "migration")
}
