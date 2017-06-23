package models

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/pkg/spec"
	"github.com/ffan/tidb-k8s/pkg/storage"
	"github.com/ffan/tidb-k8s/pkg/util/k8sutil"
)

var (
	dbS *storage.Storage

	errCellIsNil               = errors.New("cell is nil")
	errInvalidSchema           = errors.New("invalid database schema")
	errInvalidDatabaseUsername = errors.New("invalid database username")
	errInvalidDatabasePassword = errors.New("invalid database password")
)

// NewDb create a db instance
func NewDb(cell ...string) *Db {
	td := Db{}
	if len(cell) > 0 {
		td.Metadata.Name = cell[0]
	}
	return &td
}

func dbInit() {
	s, err := storage.NewStorage(getNamespace(), spec.TPRKindTidb)
	if err != nil {
		panic(fmt.Errorf("Create storage db error: %v", err))
	}
	dbS = s
}

// Save db
func (db *Db) Save() error {
	db.Metadata.Name = uniqueID(db.Owner.ID, db.Schema.Name)
	if err := db.check(); err != nil {
		return err
	}
	if old, _ := GetDb(db.Metadata.Name); old != nil {
		return fmt.Errorf(`db "%s" has created`, old.Metadata.Name)
	}
	if pods, err := k8sutil.ListPodNames(db.Metadata.Name, ""); err != nil || len(pods) > 1 {
		return fmt.Errorf(`db "%s" has not been cleared yet: %v`, db.Metadata.Name, err)
	}
	logs.Debug("db: %+v", db)

	if err := dbS.Create(db); err != nil {
		return err
	}
	return nil
}

func (db *Db) check() (err error) {
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
	md := getCachedMetadata()
	p.CPU = md.Spec.Pd.CPU
	p.Mem = md.Spec.Pd.Mem
	p.Replicas = 3
	if err := p.validate(); err != nil {
		return err
	}
	max := md.Spec.Pd.Max
	if p.Spec.Replicas < 3 || p.Spec.Replicas > max || p.Spec.Replicas%2 == 0 {
		return fmt.Errorf("replicas must be an odd number >= 3 and <= %d", max)
	}

	// set volume

	if len(p.Spec.Volume) == 0 {
		p.Spec.Volume = "emptyDir: {}"
	} else {
		p.Spec.Volume = fmt.Sprintf("hostPath: {path: %s}", p.Spec.Volume)
	}
	return nil
}

func (tk *Tikv) check() error {
	md := getCachedMetadata()
	tk.CPU = md.Spec.Tikv.CPU
	tk.Mem = md.Spec.Tikv.Mem
	if tk.Capatity < 1 {
		tk.Capatity = md.Spec.Tikv.Capacity
	}
	if err := tk.validate(); err != nil {
		return err
	}
	max := md.Spec.Tikv.Max
	if tk.Spec.Replicas < 3 || tk.Spec.Replicas > max {
		return fmt.Errorf("replicas must be >= 3 and <= %d", max)
	}
	tk.Spec.Volume = strings.Trim(md.Spec.K8s.Volume, " ")
	if len(tk.Spec.Volume) == 0 {
		tk.Spec.Volume = "emptyDir: {}"
	} else {
		tk.Spec.Volume = fmt.Sprintf("hostPath: {path: %s}", tk.Spec.Volume)
	}
	return nil
}

func (td *Tidb) check() error {
	md := getCachedMetadata()
	td.CPU = md.Spec.Tidb.CPU
	td.Mem = md.Spec.Tidb.Mem
	if err := td.validate(); err != nil {
		return err
	}
	max := md.Spec.Tidb.Max
	if td.Replicas < 1 || td.Replicas > max {
		return fmt.Errorf("replicas must be >= 1 and <= %d", max)
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

// Update db
func (db *Db) Update() error {
	return dbS.Update(db.Metadata.Name, db)
}

func (db *Db) update() error {
	return dbS.Update(db.Metadata.Name, db)
}

// GetDb get a db instance
func GetDb(cell string) (*Db, error) {
	db := NewDb()
	err := dbS.Get(cell, db)
	if err != nil {
		return nil, err
	}
	if db.Pd != nil {
		db.Pd.Db = db
	}
	if db.Tikv != nil {
		db.Tikv.Db = db
	}
	if db.Tidb != nil {
		db.Tidb.Db = db
	}
	return db, nil
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
	list := &TidbList{}
	if err := dbS.List(userID, list); err != nil {
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
	logs.Warn(`Tidb "%s" deleted`, db.Metadata.Name)
	return nil
}
