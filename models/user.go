package models

import (
	"encoding/json"
	"fmt"
	"time"

	"strings"

	"github.com/astaxie/beego/logs"
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
	userS Storage
)

// Db database
type Db struct {
	Creater
	Tidb
	Hosts []string `json:"hosts,omitempty"`
}

// Creater info
type Creater struct {
	ID         string `json:"userid"` //user
	Name       string `json:"username"`
	DatabaseID string `json:"dbid"`

	Uscale UScale `json:"uscale,omitempty"`
}

// UScale store user scale request
type UScale struct {
	Pd     int `json:"pd"`
	Tikv   int `json:"tikv"`
	Tidb   int `json:"tidb"`
	Status int `json:"status"`
}

// NewDb return a db instance
func NewDb() *Db {
	return &Db{}
}

func userInit() {
	s, err := newStorage(userNamespace)
	if err != nil {
		panic(fmt.Errorf("Create storage user error: %v", err))
	}
	userS = s
}

// Save to storage
func (d *Db) Save() (err error) {
	if err = d.beforeSave(); err != nil {
		return err
	}
	logs.Debug("creater: %+v", d.Creater)
	if err = d.Creater.save(); err != nil {
		return err
	}

	logs.Debug("tidb: %+v", d.Tidb)
	if err = d.Tidb.Save(); err != nil {
		return err
	}
	return nil
}

func (d *Db) beforeSave() (err error) {
	if td, _ := GetTidb(d.Cell); td != nil {
		return fmt.Errorf(`tidb "%s" has been created`, d.Cell)
	}
	if len(d.ID) < 1 || len(d.Cell) < 1 {
		return fmt.Errorf("user id or cell is nil")
	}
	if len(d.User) < 1 || len(d.User) > 32 {
		return fmt.Errorf("no set user")
	}
	if len(d.Password) < 1 || len(d.Password) > 32 {
		return fmt.Errorf("no set password")
	}
	if pods, err := listPodNames(d.Cell, ""); err != nil || len(pods) > 1 {
		return fmt.Errorf(`tidb "%s" has not been cleared yet: %v`, d.Cell, err)
	}
	d.TimeCreate = time.Now()
	return nil
}

func (u *Creater) save() error {
	j, err := json.Marshal(u)
	if err != nil {
		return err
	}
	key := getUserKey(u.ID, u.DatabaseID)
	if err := userS.Create(key, j); err != nil {
		return err
	}
	return nil
}

// Update user
func (u *Creater) Update() error {
	j, err := json.Marshal(u)
	if err != nil {
		return err
	}
	if err := userS.Update(getUserKey(u.ID, u.DatabaseID), j); err != nil {
		return err
	}
	return nil
}

// Delete user
func (u *Creater) Delete() error {
	if err := userS.Delete(getUserKey(u.ID, u.DatabaseID)); err != nil {
		return err
	}
	logs.Warn(`DatabaseID "%s" deleted`, u.DatabaseID)
	return nil
}

// Delete user and tidb
func (d *Db) Delete() (err error) {
	td, err := GetTidb(d.Cell)
	if err != nil {
		return err
	}
	if err = td.Delete(func() {
		if err := d.Creater.Delete(); err != nil {
			logs.Error("Delete user: %v", err)
		}
	}); err != nil {
		return err
	}
	return
}

// NeedLimitResources whether the user creates tidb for approval
func NeedLimitResources(ID string, kvr, dbr uint) bool {
	if len(ID) < 1 {
		return true
	}
	md, err := GetMetadata()
	if err != nil {
		logs.Error("cant get metadata when invoke limitResources: %v", err)
		return true
	}
	dbs, err := GetDbs(ID)
	if err != nil {
		logs.Error("cant get user %s dbs: %v", ID, err)
	}
	for _, db := range dbs {
		kvr = kvr + uint(db.Tikv.Replicas)
		dbr = dbr + uint(db.Replicas)
	}
	if kvr > md.AC.KvReplicas {
		return true
	}
	if dbr > md.AC.DbReplicas {
		return true
	}
	return false
}

// GetDbs Return the user's db, if the admin user, then return all the database
func GetDbs(ID string) ([]Db, error) {
	if len(ID) < 1 {
		return nil, fmt.Errorf("userid is nil")
	}
	if ID != "admin" {
		return getDbs(ID)
	}

	// Admin user returns so tidbs
	ids, err := userS.ListKey("")
	if err != nil && err != ErrNoNode {
		return nil, err
	}
	if len(ids) < 1 {
		return nil, nil
	}
	all := []Db{}
	for _, id := range ids {
		dbs, err := getDbs(id)
		if err != nil {
			return nil, err
		}
		all = append(all, dbs...)
	}
	return all, nil
}

func getDbs(ID string) ([]Db, error) {
	cells, err := userS.ListKey(ID)
	if err != nil && err != ErrNoNode {
		return nil, err
	}
	l := len(cells)
	if l < 1 {
		return nil, nil
	}
	dbs := make([]Db, l)
	for i, cell := range cells {
		db, err := GetDb(ID, cell)
		if err != nil {
			return nil, err
		}
		dbs[i] = *db
	}
	return dbs, nil
}

// GetDb gets the specified user's tidb
func GetDb(ID, cell string) (*Db, error) {
	db := NewDb()
	db.Creater = Creater{}
	err := userS.GetObj(getUserKey(ID, cell), &db.Creater)
	if err != nil {
		return nil, err
	}
	td, _ := GetTidb(cell)
	if td == nil {
		return db, nil
	}
	db.Tidb = *td
	if td.isNil() {
		return db, nil
	}
	if len(td.Nets) > 0 {
		// Set tidb's external hosts
		var mysqls, stats []string
		for _, n := range td.Nets {
			if n.Name == portMysql {
				mysqls = append(mysqls, n.String())
			} else {
				stats = append(stats, n.String())
			}
		}
		db.Hosts = []string{strings.Join(mysqls, ","), strings.Join(stats, ",")}
	}
	return db, nil
}

func getUserKey(ID, cell string) string {
	return fmt.Sprintf("%s/%s", ID, cell)
}
