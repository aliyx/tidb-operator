package controllers

import (
	"encoding/json"

	"fmt"

	"strings"

	"strconv"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/models"
)

// Operations about database
type UserController struct {
	beego.Controller
}

// Post 添加tidb到user
// @Title AddTidb
// @Description associate tidb to the user
// @Param	body	body 	models.Db	true	"body for user&tidb content"
// @Success 200
// @Failure 403 body is empty
// @router / [post]
func (uc *UserController) Post() {
	db := models.NewDb()
	b := uc.Ctx.Input.RequestBody
	if len(b) < 1 {
		uc.CustomAbort(403, "body is empty")
	}
	if err := json.Unmarshal(b, db); err != nil {
		uc.CustomAbort(400, fmt.Sprintf("Parse body error: %v", err))
	}
	db.Cell = uniqueID(db.ID, db.Schema)
	db.DatabaseID = db.Cell
	err := db.Save()
	if err != nil {
		logs.Error(`Save tidb "%s" to user: %v`, db.Cell, err)
		uc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf(`Save tidb "%s" to user: %v`, db.Cell, err))
	}
	// start is async
	if db.Status == models.Undefined {
		models.Start(db.Cell)
	}
	uc.Data["json"] = db.Cell
	uc.ServeJSON()
}

// Delete 删除tidb
// @Title Delete tidb
// @Description delete the tidb from user
// @Param	user	path 	string	true "The user you want to delete tidb"
// @Param	cell	path 	string	true "The cell you want to delete"
// @Success 200 {string} delete success!
// @Failure 403 user is empty
// @Failure 403 cell is empty
// @router /:user/tidbs/:cell [delete]
func (uc *UserController) Delete() {
	user := uc.GetString(":user")
	if len(user) < 1 {
		uc.CustomAbort(403, "user id is nil")
	}
	cell := uc.GetString(":cell")
	if len(cell) < 1 {
		uc.CustomAbort(403, "tidb name is nil")
	}
	db := models.NewDb()
	db.ID = user
	db.DatabaseID = cell
	db.Cell = cell
	if err := db.Delete(); err != nil {
		logs.Error(`Cannt delete tidb "%s" from user: %v`, cell, err)
		uc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf(`Cannt delete tidb "%s" from user: %v`, cell, err))
	}
	uc.Data["json"] = 1
	uc.ServeJSON()
}

// Get 返回user指定的tidb
// @Title GetTidbByUserAndCell
// @Description get tidb by user and cell
// @Param user path string true "The user id for tidb"
// @Param cell path string true "The cell for tidb"
// @Success 200 {object} models.Db
// @Failure 404 :cell not found
// @router /:user/tidbs/:cell [get]
func (uc *UserController) Get() {
	user := uc.GetString(":user")
	if len(user) < 1 {
		uc.CustomAbort(403, "user id is nil")
	}
	cell := uc.GetString(":cell")
	if len(cell) < 1 {
		uc.CustomAbort(403, "cell is nil")
	}
	db, err := models.GetDb(user, cell)
	if err != nil {
		logs.Error("Cannt get user %s'tidb %s: %v", user, cell, err)
		uc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("Cannt get user %s'tidb %s: %v", user, cell, err))
	}
	uc.Data["json"] = *db
	uc.ServeJSON()
}

// GetAll 指定用户的tidbs
// @Title GetTidbsByUser
// @Description Get tidbs by user
// @Param	user	path	string	true "The user id"
// @Success 200 {object} []Dbs
// @Failure 404 :user not found
// @router /:user/tidbs [get]
func (uc *UserController) GetAll() {
	user := uc.GetString(":user")
	if len(user) < 1 {
		uc.CustomAbort(403, "user id is nil")
	}
	dbs, err := models.GetDbs(user)
	if err != nil {
		logs.Error("Cannt get user:%s tidbs: %v", user, err)
		uc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("Cannt get user:%s tidbs: %v", user, err))
	}
	uc.Data["json"] = Dbs{len(dbs), dbs}
	uc.ServeJSON()
}

// CheckResources Check the user's request for resources
// @Title CheckResources
// @Description whether the user creates tidb for approval
// @Param 	user 	path 	string 	true	"The user id"
// @Param	body	body 	models.ApprovalConditions	true	"body for resource content"
// @Success 200
// @Failure 403 body is empty
// @router /:user/limit [post]
func (uc *UserController) CheckResources() {
	user := uc.GetString(":user")
	if len(user) < 1 {
		uc.CustomAbort(403, "user id is nil")
	}
	ac := &models.ApprovalConditions{}
	b := uc.Ctx.Input.RequestBody
	if len(b) < 1 {
		uc.CustomAbort(403, "body is empty")
	}
	if err := json.Unmarshal(b, ac); err != nil {
		uc.CustomAbort(400, fmt.Sprintf("Parse body error: %v", err))
	}
	limit := models.NeedLimitResources(user, ac.KvReplicas, ac.DbReplicas)
	uc.Data["json"] = limit
	uc.ServeJSON()
}

// Status modify db status
// @Title status
// @Description modify db status
// @Param 	user 	path 	string 	true	"The user id"
// @Param	cell	path	string	true	"The cell for tidb name"
// @Param	body	body 	status	true	"The body data type is {'type': string, status': string}"
// @Success 200
// @Failure 400 body is empty
// @Failure 403 unsupport operation
// @router /:user/tidbs/:cell/status [patch]
func (uc *UserController) Status() {
	ID := uc.GetString(":user")
	if len(ID) < 1 {
		uc.CustomAbort(403, "user id is nil")
	}
	cell := uc.GetString(":cell")
	if len(cell) < 1 {
		uc.CustomAbort(403, "cell is nil")
	}
	var err error
	s := status{}
	if err = json.Unmarshal(uc.Ctx.Input.RequestBody, &s); err != nil {
		uc.CustomAbort(400, fmt.Sprintf("Parse body for patch error: %v", err))
	}
	switch s.Type {
	case "roll":
		var db *models.Db
		var st int
		if db, err = models.GetDb(ID, cell); err != nil {
			errHandler(uc.Controller, err, "get db")
		}
		if st, err = strconv.Atoi(s.Status); err != nil {
			errHandler(uc.Controller, err, "conv tidb status string to int")
		}
		db.Status = models.TidbStatus(st)
		db.Reason = s.Desc
		if err = db.Update(); err != nil {
			errHandler(uc.Controller, err, "update tidb status")
		}
	default:
		uc.CustomAbort(403, "unsupport status")
	}
}

// Dbs db array
type Dbs struct {
	Total int         `json:"total"`
	Tidbs []models.Db `json:"tidbs"`
}

func uniqueID(uid, schema string) string {
	var u string
	if i, err := strconv.ParseInt(uid, 10, 32); err == nil {
		u = fmt.Sprintf("%03x", i)
	} else {
		u = fmt.Sprintf("%03s", uid)
	}
	return strings.ToLower(fmt.Sprintf("%s-%s", u[len(u)-3:], strings.Replace(schema, "_", "-", -1)))
}
