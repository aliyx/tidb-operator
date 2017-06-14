package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/astaxie/beego/logs"

	"github.com/ffan/tidb-k8s/pkg/k8sutil"
	"github.com/ffan/tidb-k8s/pkg/retryutil"
	"github.com/ffan/tidb-k8s/pkg/storage"

	"errors"
	"sync"

	"strconv"

	"github.com/ffan/tidb-k8s/pkg/httputil"
	"github.com/ghodss/yaml"
)

// Phase tidb runing status
type Phase int

const (
	// Refuse user apply create a tidb
	Refuse Phase = iota - 2
	// Auditing wait admin audit
	Auditing
	// Undefined wait install tidb
	Undefined
	pdPending
	pdStartFailed
	pdStarted
	tikvPending
	tikvStartFailed
	tikvStarted
	tidbPending
	tidbStartFailed
	tidbStarted
	tidbInitFailed
	// tidbInited 初始化完成，可对外提供服务
	tidbInited
	tidbStopFailed
	tidbStoped
	tikvStopFailed
	tikvStoped
	pdStopFailed
	pdStoped
	tidbDeleting
)

const (
	migrating          = "Migrating"
	migStartMigrateErr = "StartMigrationTaskError"

	stopTidbTimeout                   = 60 // 60s
	waitPodRuningTimeout              = 30 * time.Second
	waitTidbComponentAvailableTimeout = 60 * time.Second

	scaling      = 1 << 8
	tikvScaleErr = 1
	tidbScaleErr = 1 << 1
)

const (
	// ScaleUndefined no scale request
	ScaleUndefined int = iota
	// ScalePending wait for the admin to scale
	ScalePending
	// ScaleFailure scale failure
	ScaleFailure
	// Scaled scale success
	Scaled
)

var (
	tidbS storage.Storage

	errCellIsNil               = errors.New("cell is nil")
	errInvalidSchema           = errors.New("invalid database schema")
	errInvalidDatabaseUsername = errors.New("invalid database username")
	errInvalidDatabasePassword = errors.New("invalid database password")

	// ErrRepeatOperation is returned by functions to specify the operation is executing.
	ErrRepeatOperation = errors.New("the previous operation is being executed, please stop first")
	errInvalidReplica  = errors.New("invalid replica")

	scaleMu sync.Mutex
)

// Tidb metadata
type Tidb struct {
	Cell    string   `json:"cell"`
	Owner   *Owner   `json:"owner,omitempty"`
	Schemas []Schema `json:"schemas"`
	Spec    Spec     `json:"spec"`
	Pd      *Pd      `json:"pd"`
	Tikv    *Tikv    `json:"tikv"`

	Status               Status    `json:"status"`
	OuterAddresses       []string  `json:"outerAddresses,omitempty"`
	OuterStatusAddresses []string  `json:"outerStatusAddresses,omitempty"`
	CreationTimestamp    time.Time `json:"creationTimestamp,omitempty"`
}

// Owner creater
type Owner struct {
	ID     string `json:"userId"` //user
	Name   string `json:"userName"`
	Desc   string `json:"desc,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// Schema database schema
type Schema struct {
	Name     string `json:"name"`
	User     string `json:"user"`
	Password string `json:"password"`
}

// Spec describe a pd/tikv/tidb specification
type Spec struct {
	CPU      int    `json:"cpu"`
	Mem      int    `json:"mem"`
	Version  string `json:"version"`
	Replicas int    `json:"replicas"`
	Volume   string `json:"tidbdata_volume,omitempty"`
	Capatity int    `json:"capatity,omitempty"`
}

// Validate cpu or mem is effective
func (s *Spec) validate() error {
	if s.CPU < 200 || s.CPU > 2000 {
		return fmt.Errorf("cpu must be between 200-2000")
	}
	if s.Mem < 256 || s.CPU > 8184 {
		return fmt.Errorf("cpu must be between 256-8184")
	}
	if s.Replicas < 1 {
		return fmt.Errorf("replicas must be greater than 1")
	}
	if s.Version == "" {
		return fmt.Errorf("please specify image version")
	}
	return nil
}

// Status tidb status
type Status struct {
	Available    bool   `json:"available"`
	Phase        Phase  `json:"phase"`
	MigrateState string `json:"migrateState"`
	ScaleState   int    `json:"scaleState"`
	Desc         string `json:"desc,omitempty"`
}

func tidbInit() {
	s, err := storage.NewDefaultStorage(tidbNamespace, etcdAddress)
	if err != nil {
		panic(fmt.Errorf("Create storage tidb error: %v", err))
	}
	tidbS = s
}

// NewTidb create a tidb instance
func NewTidb(cell ...string) *Tidb {
	td := Tidb{}
	if len(cell) > 0 {
		td.Cell = cell[0]
	}
	return &td
}

// Save tidb
func (db *Tidb) Save() error {
	db.Cell = uniqueID(db.Owner.ID, db.Schemas[0].Name)
	if err := db.check(); err != nil {
		return err
	}
	if old, _ := GetTidb(db.Cell); old != nil {
		return fmt.Errorf(`tidb "%s" has created`, old.Cell)
	}
	if pods, err := k8sutil.ListPodNames(db.Cell, ""); err != nil || len(pods) > 1 {
		return fmt.Errorf(`tidb "%s" has not been cleared yet: %v`, db.Cell, err)
	}
	db.CreationTimestamp = time.Now()
	logs.Debug("tidb: %+v", db)

	j, err := json.Marshal(db)
	if err != nil {
		return err
	}
	if err := tidbS.Create(db.Cell, j); err != nil {
		return err
	}
	return nil
}

func (db *Tidb) check() (err error) {
	if err = db.Spec.validate(); err != nil {
		return err
	}
	for _, s := range db.Schemas {
		if len(s.Name) < 1 || len(s.Name) > 32 {
			return errInvalidSchema
		}
		if len(s.User) < 1 || len(s.User) > 32 {
			return errInvalidDatabaseUsername
		}
		if len(s.Password) < 1 || len(s.Password) > 32 {
			return errInvalidDatabasePassword
		}
	}
	if err = db.Pd.beforeSave(); err != nil {
		return err
	}
	if err = db.Tikv.beforeSave(); err != nil {
		return err
	}
	return nil
}

func uniqueID(uid, schema string) string {
	u := encodeUserID(uid)
	return strings.ToLower(fmt.Sprintf("%s-%s", u, strings.Replace(schema, "_", "-", -1)))
}

func encodeUserID(uid string) string {
	var u string
	if i, err := strconv.ParseInt(uid, 10, 32); err == nil {
		u = fmt.Sprintf("%03x", i)
	} else {
		u = fmt.Sprintf("%03s", uid)
	}
	return u[len(u)-3:]
}

// Update tidb
func (db *Tidb) Update() error {
	if db.Cell == "" {
		return errCellIsNil
	}
	if err := db.check(); err != nil {
		return err
	}
	return db.update()
}

func (db *Tidb) update() error {
	j, err := json.Marshal(db)
	if err != nil {
		return err
	}
	return tidbS.Update(db.Cell, j)
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

// GetDbs get specified user tidbs
func GetDbs(userID string) ([]Tidb, error) {
	if len(userID) < 1 {
		return nil, fmt.Errorf("userid is nil")
	}
	var (
		err   error
		cells []string
	)
	if userID != "admin" {
		cells, err = tidbS.ListKey(encodeUserID(userID) + "-")
	} else {
		cells, err = tidbS.ListDir("")
	}
	if err != nil && err != storage.ErrNoNode {
		return nil, err
	}
	if len(cells) < 1 {
		return nil, nil
	}
	all := []Tidb{}
	for _, cell := range cells {
		db, err := GetTidb(cell)
		if err != nil {
			return nil, err
		}
		all = append(all, *db)
	}
	return all, nil
}

// NeedLimitResources whether the user creates tidb for approval
func NeedLimitResources(ID string, kvr, dbr uint) bool {
	if len(ID) < 1 {
		return true
	}
	dbs, err := GetDbs(ID)
	if err != nil {
		logs.Error("cant get user %s dbs: %v", ID, err)
	}
	for _, db := range dbs {
		kvr = kvr + uint(db.Tikv.Spec.Replicas)
		dbr = dbr + uint(db.Spec.Replicas)
	}
	md := getCachedMetadata()
	if kvr > md.AC.KvReplicas {
		return true
	}
	if dbr > md.AC.DbReplicas {
		return true
	}
	return false
}

func isOkPd(cell string) bool {
	if db, err := GetTidb(cell); err != nil ||
		db == nil || db.Status.Phase < pdStarted || db.Status.Phase > tikvStoped {
		return false
	}
	return true
}

func isOkTikv(cell string) bool {
	if db, err := GetTidb(cell); err != nil ||
		db == nil || db.Status.Phase < tikvStarted || db.Status.Phase > tidbStoped {
		return false
	}
	return true
}

func (db *Tidb) install() (err error) {
	e := NewEvent(db.Cell, "tidb", "install")
	db.Status.Phase = tidbPending
	db.update()
	defer func() {
		ph := tidbStarted
		if err != nil {
			ph = tidbStartFailed
		}
		db.Status.Phase = ph
		err = db.update()
		e.Trace(err, fmt.Sprintf("Install tidb replicationcontrollers with %d replicas on k8s", db.Spec.Replicas))
	}()
	if err = db.createService(); err != nil {
		return err
	}
	if err = db.createReplicationController(); err != nil {
		return err
	}
	// wait tidb started
	if err = db.waitForOk(); err != nil {
		return err
	}
	return nil
}

func (db *Tidb) createService() (err error) {
	j, err := db.toJSONTemplate(tidbServiceYaml)
	if err != nil {
		return err
	}
	srv, err := k8sutil.CreateServiceByJSON(j)
	if err != nil {
		return err
	}
	ps := getProxys()
	for _, py := range ps {
		db.OuterAddresses = append(db.OuterAddresses, fmt.Sprintf("%s:%d", py, srv.Spec.Ports[0].NodePort))
	}
	db.OuterStatusAddresses = append(db.OuterStatusAddresses,
		fmt.Sprintf("%s:%d", ps[0], srv.Spec.Ports[1].NodePort))
	logs.Info("tidb %s mysql address: %s, status address: %s", db.Cell, db.OuterAddresses, db.OuterStatusAddresses)
	return nil
}

func (db *Tidb) createReplicationController() (err error) {
	j, err := db.toJSONTemplate(tidbRcYaml)
	if err != nil {
		return err
	}
	_, err = k8sutil.CreateRcByJSON(j, waitPodRuningTimeout)
	return err
}

func (db *Tidb) toJSONTemplate(temp string) ([]byte, error) {
	r := strings.NewReplacer(
		"{{version}}", db.Spec.Version,
		"{{cpu}}", fmt.Sprintf("%v", db.Spec.CPU), "{{mem}}", fmt.Sprintf("%v", db.Spec.Mem),
		"{{namespace}}", getNamespace(),
		"{{replicas}}", fmt.Sprintf("%v", db.Spec.Replicas),
		"{{registry}}", imageRegistry, "{{cell}}", db.Cell)
	str := r.Replace(temp)
	j, err := yaml.YAMLToJSON([]byte(str))
	if err != nil {
		return nil, err
	}
	return j, nil
}

func (db *Tidb) waitForOk() (err error) {
	logs.Info("waiting for run tidb %s ok...", db.Cell)
	interval := 3 * time.Second
	sURL := fmt.Sprintf("http://%s/status", db.OuterStatusAddresses[0])
	err = retryutil.Retry(interval, int(waitTidbComponentAvailableTimeout/(interval)), func() (bool, error) {
		if _, err := httputil.Get(sURL, 2*time.Second); err != nil {
			logs.Warn("get tidb status: %v", err)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		logs.Error("wait tidb %s available: %v", db.Cell, err)
	} else {
		logs.Info("tidb %s ok", db.Cell)
	}
	return err
}

func (db *Tidb) uninstall() (err error) {
	e := NewEvent(db.Cell, "tidb", "uninstall")
	defer func() {
		ph := tidbStoped
		if err != nil {
			ph = tidbStopFailed
		}
		db.Status.Phase = ph
		db.Status.MigrateState = ""
		db.Status.ScaleState = 0
		db.OuterAddresses = nil
		db.OuterStatusAddresses = nil
		err = db.update()
		if err != nil {
			logs.Error("uninstall tidb %s: %v", db.Cell, err)
		}
		e.Trace(err, "Uninstall tidb replicationcontrollers")
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
	if err = Uninstall(db.Cell, nil); err != nil {
		return err
	}
	if err = delEventsBy(db.Cell); err != nil {
		return err
	}
	go func() {
		db.Status.Phase = tidbDeleting
		db.update()
		for {
			if !started(db.Cell) {
				if err := db.delete(); err != nil && err != storage.ErrNoNode {
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
	var db *Tidb
	if db, err = GetTidb(cell); err != nil {
		return err
	}
	if db.Status.Available {
		return fmt.Errorf("tidb %s unavailable", cell)
	}
	if db.Status.ScaleState&scaling > 0 {
		return fmt.Errorf("tidb %s is scaling", cell)
	}
	db.Status.ScaleState |= scaling
	db.update()
	var wg sync.WaitGroup
	db.scaleTikvs(kvReplica, &wg)
	db.scaleTidbs(dbReplica, &wg)
	go func() {
		wg.Wait()
		db.Status.ScaleState ^= scaling
		db.update()
	}()
	return nil
}

func (db *Tidb) scaleTidbs(replica int, wg *sync.WaitGroup) {
	if replica < 1 {
		return
	}
	if replica == db.Spec.Replicas {
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
				db.Status.ScaleState |= tidbScaleErr
			}
			db.update()
			e.Trace(err, fmt.Sprintf(`Scale tidb "%s" replica: %d->%d`, db.Cell, r, replica))
		}(db.Spec.Replicas)
		md := getCachedMetadata()
		if replica > md.Units.Tidb.Max {
			err = fmt.Errorf("the replicas of tidb exceeds max %d", md.Units.Tidb.Max)
			return
		}
		if replica > db.Spec.Replicas*3 || db.Spec.Replicas > replica*3 {
			err = fmt.Errorf("each scale can not more or less then 2 times")
			return
		}
		old := db.Spec.Replicas
		db.Spec.Replicas = replica
		if err = db.Spec.validate(); err != nil {
			db.Spec.Replicas = old
			return
		}
		logs.Info(`Scale "tidb-%s" from %d to %d`, db.Cell, db.Spec.Replicas, replica)
		if err = k8sutil.ScaleReplicationController(fmt.Sprintf("tidb-%s", db.Cell), replica); err != nil {
			return
		}
	}()
}

func (db *Tidb) isNil() bool {
	return db.Spec.Replicas < 1
}

func (db *Tidb) isOk() bool {
	if db.Status.Phase < tidbStarted || db.Status.Phase > tidbInited {
		return false
	}
	return true
}
