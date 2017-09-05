package operator

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"sync"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/spec"
	"github.com/ffan/tidb-operator/pkg/storage"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewDb create a db instance
func NewDb(cell ...string) *Db {
	td := Db{}
	if len(cell) > 0 {
		td.Metadata.Name = cell[0]
	}
	return &td
}

// Save db
func (db *Db) Save() error {
	mu.Lock()
	defer mu.Unlock()
	db.Metadata.Name = uniqueID(db.Owner.ID, db.Schema.Name)
	db.TypeMeta = metav1.TypeMeta{
		Kind:       spec.CRDKindTidb,
		APIVersion: spec.SchemeGroupVersion.String(),
	}
	if old, _ := GetDb(db.GetName()); old != nil {
		return storage.ErrAlreadyExists
	}
	if pods, err := k8sutil.ListPodNames(db.GetName(), ""); err != nil || len(pods) > 1 {
		return err
	} else if len(pods) > 1 {
		return fmt.Errorf("db %q has not been cleared yet", db.GetName())
	}
	if err := db.check(); err != nil {
		return err
	}
	if err := dbS.Create(db); err != nil {
		return err
	}

	lockers[db.GetName()] = new(sync.Mutex)

	if db.Status.Phase == PhaseUndefined {
		go db.Install(true)
	}
	return nil
}

func (db *Db) check() (err error) {
	if db.Pd == nil {
		db.Pd = &Pd{}
	}
	if db.Tikv == nil {
		db.Tikv = &Tikv{}
	}
	if db.Tidb == nil {
		db.Tidb = &Tidb{}
	}
	if err = db.Schema.check(); err != nil {
		return err
	}
	if err = db.Pd.check(); err != nil {
		return err
	}
	if err = db.Tikv.check(); err != nil {
		return err
	}
	if err = db.Tidb.check(); err != nil {
		return err
	}
	return nil
}

func (s Schema) check() error {
	if len(s.Name) < 1 || len(s.Name) > 32 {
		return errInvalidSchema
	}
	if len(s.User) < 1 || len(s.User) > 32 {
		return errInvalidDatabaseUsername
	}
	if len(s.Password) < 1 || len(s.Password) > 32 {
		return errInvalidDatabasePassword
	}
	return nil
}

func (p *Pd) check() error {
	md := getNonNullMetadata()
	p.CPU = md.Pd.CPU
	p.Mem = md.Pd.Mem
	p.Replicas = 3
	if p.Version == "" {
		p.Version = defaultImageVersion
	}
	if err := p.validate(); err != nil {
		return err
	}
	max := md.Pd.Max
	if p.Spec.Replicas < 3 || p.Spec.Replicas > max || p.Spec.Replicas%2 == 0 {
		return fmt.Errorf("replicas must be an odd number >= 3 and <= %d", max)
	}
	return nil
}

func (tk *Tikv) check() error {
	md := getNonNullMetadata()
	tk.CPU = md.Tikv.CPU
	tk.Mem = md.Tikv.Mem
	if tk.Capatity < 1 {
		tk.Capatity = md.Tikv.Capacity
	}
	if tk.Replicas < 3 {
		tk.Replicas = 3
	}
	if tk.Version == "" {
		tk.Version = defaultImageVersion
	}
	if err := tk.validate(); err != nil {
		return err
	}
	max := md.Tikv.Max
	if tk.Spec.Replicas < 3 || tk.Spec.Replicas > max {
		return fmt.Errorf("replicas count must be >= 3 and <= %d", max)
	}
	tk.Volume = strings.Trim(md.K8sConfig.HostPath, " ")
	tk.Mount = md.K8sConfig.Mount
	return nil
}

func (td *Tidb) check() error {
	md := getNonNullMetadata()
	td.CPU = md.Tidb.CPU
	td.Mem = md.Tidb.Mem
	if td.Replicas < 2 {
		td.Replicas = 2
	}
	if td.Version == "" {
		td.Version = defaultImageVersion
	}
	if err := td.validate(); err != nil {
		return err
	}
	max := md.Tidb.Max
	if td.Replicas < 2 || td.Replicas > max {
		return fmt.Errorf("replicas count must be >= 2 and <= %d", max)
	}
	return nil
}

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

func (db *Db) update() error {
	return dbS.RetryUpdate(db.GetName(), db)
}

// patch all of the asynchronous processing should call the function
func (db *Db) patch(patchFunc func(*Db)) error {
	retryCount := 0
	for {
		old := NewDb()
		err := dbS.Get(db.GetName(), old)
		if err != nil {
			return err
		}
		db.Metadata.SetResourceVersion(old.Metadata.GetResourceVersion())

		// upgrade version is immutable
		db.Pd.Version = old.Pd.Version
		db.Tikv.Version = old.Tikv.Version
		db.Tidb.Version = old.Tidb.Version

		// scale replicas is immutable
		db.Status.ScaleCount = old.Status.ScaleCount
		db.Tikv.Replicas = old.Tikv.Replicas
		db.Tidb.Replicas = old.Tidb.Replicas

		// migrate maybe will change
		db.Status.MigrateState = old.Status.MigrateState
		db.Status.MigrateRetryCount = old.Status.MigrateRetryCount
		db.Status.Reason = old.Status.Reason
		if patchFunc != nil {
			patchFunc(db)
		}

		err = dbS.Update(db.GetName(), db)
		if err == storage.ErrConflict {
			if retryCount > 5 {
				logs.Error("retry update db %q over %d times, exit", db.GetName(), retryCount)
				return err
			}
			retryCount++
			continue
		}
		return nil
	}
}

// GetDb get a db instance
func GetDb(cell string) (*Db, error) {
	db := NewDb()
	err := dbS.Get(cell, db)
	if err != nil {
		return nil, err
	}
	db.AfterPropertiesSet()
	return db, nil
}

// AfterPropertiesSet ...
func (db *Db) AfterPropertiesSet() {
	db.Pd.Db = db
	db.Tikv.Db = db
	db.Tidb.Db = db
}

//Clone ...
func (db *Db) Clone() *Db {
	c := *db
	var nD = &c
	pd := *(db.Pd)
	nD.Pd = &pd
	tk := *(db.Tikv)
	nD.Tikv = &tk
	td := *(db.Tidb)
	nD.Tidb = &td
	nD.AfterPropertiesSet()
	return nD
}

// Unmarshal ...
func (db *Db) Unmarshal(data []byte) error {
	if err := json.Unmarshal(data, db); err != nil {
		return err
	}
	db.AfterPropertiesSet()
	return nil
}

// GetAllDbs get a dbList object
func GetAllDbs() (*DbList, error) {
	list := &DbList{}
	if err := dbS.List(list); err != nil {
		if err != storage.ErrNoNode {
			return nil, err
		}
		return nil, nil
	}
	return list, nil
}

// GetDbs get specified user dbs
func GetDbs(userID string) ([]Db, error) {
	if len(userID) < 1 {
		return nil, fmt.Errorf("userid is nil")
	}
	if userID != "admin" {
		userID = encodeUserID(userID) + "-"
	} else {
		userID = ""
	}
	list := &DbList{}
	if err := dbS.List(list); err != nil {
		if err != storage.ErrNoNode {
			return nil, err
		}
		return nil, nil
	}
	var all []Db
	for _, db := range list.Items {
		if strings.HasPrefix(db.Metadata.Name, userID) {
			all = append(all, db)
		}
	}
	return all, nil
}

func (db *Db) delete() error {
	if err := dbS.Delete(db.Metadata.Name); err != nil {
		return err
	}
	logs.Info("tidb %q deleted", db.Metadata.Name)
	return nil
}
