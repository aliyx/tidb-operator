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
	statAPI = "http://%s:%d/tidb/api/v1/tidbs/%s"
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
		db.Install(nil)
	}
	dc.Data["json"] = db.GetName()
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

// Limit Check the user's request for resources
// @Title Limit
// @Description Whether the user creates tidb for approval
// @Param 	user 	path 	string 	true	"The user id"
// @Param	body	body 	operator.ApprovalConditions	true	"body for resource content"
// @Success 200
// @Failure 403 body is empty
// @router /:user/limit [post]
func (dc *TidbController) Limit() {
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

// Patch tidb
// @Title Patch
// @Description partially update the specified Tidb
// @Param	cell	path	string	true	"The cell for pd name"
// @Param	body	body	operator.Db	true	"Data format reference: http://jsonpatch.com/"
// @Success 200
// @Failure 403 body is empty
// @Failure 403 unsupport operation
// @router /:cell [patch]
func (dc *TidbController) Patch() {
	cell := dc.GetString(":cell")
	b := dc.Ctx.Input.RequestBody
	if len(b) < 1 {
		dc.CustomAbort(403, "body is empty")
	}
	db, err := operator.GetDb(cell)
	errHandler(dc.Controller, err, fmt.Sprintf("get db %s", cell))

	newDb := db.Clone()
	if err = patch(b, newDb); err != nil {
		dc.CustomAbort(400, fmt.Sprintf("parse patch body err: %v", err))
	}
	switch db.Operator {
	case "patch":
		newDb.Update()
	case "audit":
		switch db.Status.Phase {
		case operator.PhaseRefuse:
			newDb.Update()
		case operator.PhaseUndefined:
			errHandler(
				dc.Controller,
				newDb.Install(nil),
				fmt.Sprintf("start installing tidb %s", cell),
			)
		}
	case "start":
		errHandler(
			dc.Controller,
			newDb.Install(nil),
			fmt.Sprintf("start installing tidb %s", cell),
		)
	case "stop":
		errHandler(
			dc.Controller,
			newDb.Uninstall(nil),
			fmt.Sprintf("start uninstalling tidb %s", cell),
		)
	case "retart":
		errHandler(
			dc.Controller,
			newDb.Reinstall(cell),
			fmt.Sprintf("start reinstalling tidb %s", cell),
		)
	case "upgrade":
		errHandler(
			dc.Controller,
			newDb.Upgrade(),
			fmt.Sprintf("upgrade tidb %s", cell),
		)
	case "scale":
		errHandler(
			dc.Controller,
			db.Scale(newDb.Tikv.Replicas, newDb.Tidb.Replicas),
			fmt.Sprintf("Scale tidb %s", cell),
		)
	case "syncMigrateStat":
		db.SyncMigrateStat()
		errHandler(dc.Controller, err, "sync tidb migrate status")
	default:
		dc.CustomAbort(403, "unsupport operation")
	}
}

// Get a tidb
// @Title Get a tidb
// @Description get tidb by cell
// @Param	cell	path	string	true	"The cell for tidb name"
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
	errHandler(dc.Controller, err, fmt.Sprintf("get %s events", cell))

	dc.Data["json"] = es
	dc.ServeJSON()
}

// @Title Migrate
// @Description migrate mysql data to tidb
// @Param   sync	query	string	false	"increment sync"
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
	b := dc.Ctx.Input.RequestBody
	if len(b) < 1 {
		dc.CustomAbort(403, "Body is empty")
	}
	src := &mysqlutil.Mysql{}
	if err := json.Unmarshal(b, src); err != nil {
		dc.CustomAbort(400, fmt.Sprintf("Parse body error: %v", err))
	}
	db, err := operator.GetDb(cell)
	errHandler(dc.Controller, err, fmt.Sprintf("get db %s", cell))

	api := fmt.Sprintf(statAPI, beego.BConfig.Listen.HTTPAddr, beego.BConfig.Listen.HTTPPort, cell)
	errHandler(
		dc.Controller,
		db.Migrate(*src, api, sync == "true"),
		fmt.Sprintf(`Migrate mysql "%s" to tidb `, cell),
	)
}

func errHandler(c beego.Controller, err error, msg string) {
	if err == nil {
		logs.Info("%s ok.", msg)
		return
	}
	logs.Error("%s: %v", msg, err)
	c.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("%s error: %v", msg, err))
}
