package controllers

import (
	"encoding/json"

	"fmt"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/operator"
	"github.com/ffan/tidb-operator/pkg/util/mysqlutil"
)

var (
	// advertiseIP=beego.BConfig.Listen.HTTPSAddr
	statAPI = "%s:%d/tidb/api/v1/tidbs/%s/status"
)

// TidbController operations about tidb
type TidbController struct {
	beego.Controller
}

// Post create a tidb instance, and asynchronously install if the Phase is PhaseUndefined.
// @Title CreateTidb
// @Description create a tidb
// @Param	body	body	operator.Db	true	"body for tidb content"
// @Success 200
// @Failure 403 body is empty
// @router / [post]
func (dc *TidbController) Post() {
	b := dc.Ctx.Input.RequestBody
	if len(b) < 1 {
		dc.CustomAbort(403, "body is empty")
	}
	db := operator.NewDb()
	if err := json.Unmarshal(b, db); err != nil {
		dc.CustomAbort(400, fmt.Sprintf("parse body %v", err))
	}
	errHandler(
		dc.Controller,
		db.Save(),
		fmt.Sprintf("Create tidb %s", db.Schema.Name),
	)
	// start is async
	if db.Status.Phase == operator.PhaseUndefined {
		operator.Install(db.Metadata.Name, nil)
	}
	dc.Data["json"] = db.Metadata.Name
	dc.ServeJSON()
}

// Delete delete a tidb install
// @Title DeleteTidb
// @Description delete the tidb instance
// @Param	cell	path 	string	true "The cell you want to delete"
// @Success 200 {string} delete success!
// @Failure 403 cell is empty
// @router /:cell [delete]
func (dc *TidbController) Delete() {
	cell := dc.GetString(":cell")
	if len(cell) < 1 {
		dc.CustomAbort(403, "tidb name is nil")
	}
	errHandler(
		dc.Controller,
		operator.Delete(cell),
		fmt.Sprintf("delete tidb %s", cell),
	)
	dc.Data["json"] = 1
	dc.ServeJSON()
}

// Get 获取tidb数据
// @Title Get
// @Description get tidb by cell
// @Param cell path string true "The cell for tidb name"
// @Success 200 {object} operator.Db
// @Failure 404 :key not found
// @router /:cell [get]
func (dc *TidbController) Get() {
	cell := dc.GetString(":cell")
	db, err := operator.GetDb(cell)
	errHandler(
		dc.Controller,
		err,
		fmt.Sprintf("Cannt get tidb %s", cell),
	)
	dc.Data["json"] = db
	dc.ServeJSON()
}

// CheckResources Check the user's request for resources
// @Title CheckResources
// @Description whether the user creates tidb for approval
// @Param 	user 	path 	string 	true	"The user id"
// @Param	body	body 	operator.ApprovalConditions	true	"body for resource content"
// @Success 200
// @Failure 403 body is empty
// @router /:user/limit [post]
func (dc *TidbController) CheckResources() {
	user := dc.GetString(":user")
	if len(user) < 1 {
		dc.CustomAbort(403, "user id is nil")
	}
	ac := &operator.ApprovalConditions{}
	b := dc.Ctx.Input.RequestBody
	if len(b) < 1 {
		dc.CustomAbort(403, "body is empty")
	}
	if err := json.Unmarshal(b, ac); err != nil {
		dc.CustomAbort(400, fmt.Sprintf("Parse body error: %v", err))
	}
	limit := operator.Limit(user, ac.KvReplicas, ac.DbReplicas)
	dc.Data["json"] = limit
	dc.ServeJSON()
}

// Patch scale tidb
// @Title ScaleTidb
// @Description scale tidb
// @Param	cell	path	string	true	"The cell for pd name"
// @Param	body	body 	controllers.ScaleReq	true	"body for patch content"
// @Success 200
// @Failure 403 body is empty
// @router /:cell/scale [patch]
func (dc *TidbController) Patch() {
	cell := dc.GetString(":cell")
	s := ScaleReq{}
	if err := json.Unmarshal(dc.Ctx.Input.RequestBody, &s); err != nil {
		dc.CustomAbort(400, fmt.Sprintf("Parse body for patch error: %v", err))
	}
	errHandler(
		dc.Controller,
		operator.Scale(cell, s.KvReplica, s.DbReplica),
		fmt.Sprintf("Scale tidb %s", cell),
	)
}

// Migrate data to tidb
// @Title Migrate
// @Description migrate mysql data to tidb
// @Param   sync	query	string	false       "increment sync"
// @Param 	cell 	path	string	true	"The database name for tidb"
// @Param	body	body	mysqlutil.Mysql	true	"Body for src mysql"
// @Success 200
// @Failure 403 body is empty
// @router /:cell/migrate [post]
func (dc *TidbController) Migrate() {
	cell := dc.GetString(":cell")
	if len(cell) < 1 {
		dc.CustomAbort(403, "cell is nil")
	}
	sync := dc.GetString("sync")
	src := &mysqlutil.Mysql{}
	b := dc.Ctx.Input.RequestBody
	if len(b) < 1 {
		dc.CustomAbort(403, "Body is empty")
	}
	if err := json.Unmarshal(b, src); err != nil {
		dc.CustomAbort(400, fmt.Sprintf("Parse body error: %v", err))
	}
	db, err := operator.GetDb(cell)
	if err != nil {
		dc.CustomAbort(404, fmt.Sprintf("Cannt get tidb: %v", err))
	}
	api := fmt.Sprintf(statAPI, beego.BConfig.Listen.HTTPAddr, beego.BConfig.Listen.HTTPPort, cell)
	errHandler(
		dc.Controller,
		db.Migrate(*src, api, sync == "true"),
		fmt.Sprintf(`Migrate mysql "%s" to tidb `, cell),
	)
}

// GetEvents get events
// @Title GetEvents
// @Description get all events
// @Param	cell	path	string	true	"The cell for tidb name"
// @Success 200 {object} operator.Events
// @router /:cell/events [get]
func (dc *TidbController) GetEvents() {
	cell := dc.GetString(":cell")
	if len(cell) < 1 {
		dc.CustomAbort(403, "cell is nil")
	}
	es, err := operator.GetEventsBy(cell)
	errHandler(
		dc.Controller,
		err,
		fmt.Sprintf("get %s events", cell),
	)
	dc.Data["json"] = es
	dc.ServeJSON()
}

// Status patch tidb status
// @Title status
// @Description patch tidb status
// @Param	cell	path	string	true	"The cell for pd name"
// @Param	body	body	controllers.Status	true	"Patch tidb status"
// @Success 200
// @Failure 400 body is empty
// @Failure 403 unsupport operation
// @router /:cell/status [patch]
func (dc *TidbController) Status() {
	cell := dc.GetString(":cell")
	s := Status{}
	if err := json.Unmarshal(dc.Ctx.Input.RequestBody, &s); err != nil {
		dc.CustomAbort(400, fmt.Sprintf("Parse body for patch error: %v", err))
	}
	db, err := operator.GetDb(cell)
	errHandler(dc.Controller, err, fmt.Sprintf("Get tidb %s", cell))
	logs.Debug("%s patch: %+v", cell, s)
	switch s.Type {
	case "migrate":
		db.UpdateMigrateStat(s.Status, "")
		errHandler(dc.Controller, err, "Patch tidb status")
	case "audit":
		switch s.Status {
		case "-2":
			db.Status.Phase = operator.PhaseRefuse
			db.Owner.Reason = s.Desc
			db.Update()
		}
	case "op":
		switch s.Status {
		case "start":
			errHandler(
				dc.Controller,
				operator.Install(cell, nil),
				fmt.Sprintf("Start installing tidb %s", cell),
			)
		case "stop":
			errHandler(
				dc.Controller,
				operator.Uninstall(cell, nil),
				fmt.Sprintf("Start uninstalling tidb %s", cell),
			)
		case "retart":
			errHandler(
				dc.Controller,
				operator.Reinstall(cell),
				fmt.Sprintf("Start reinstalling tidb %s", cell),
			)
		default:
			dc.CustomAbort(403, "unsupport operation")
		}
	default:
		dc.CustomAbort(403, "unsupport operation")
	}
}

func errHandler(c beego.Controller, err error, msg string) {
	if err == nil {
		logs.Info("%s ok.", msg)
		return
	}
	logs.Error("%s: %v", msg, err)
	c.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("%s error: %v", msg, err))
}

// Status patch tidb status
type Status struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	Desc   string `json:"desc"`
}

// ScaleReq tidb scale request
type ScaleReq struct {
	DbReplica int `json:"dbReplica"`
	KvReplica int `json:"kvReplica"`
}
