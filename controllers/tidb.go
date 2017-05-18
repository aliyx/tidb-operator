package controllers

import (
	"encoding/json"

	"fmt"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/models"
	"github.com/ffan/tidb-k8s/mysql"
)

// Operations about tidb
type TidbController struct {
	beego.Controller
}

// Post 创建tidb服务,返回对外ip和port
// @Title CreateTidb
// @Description create tidb
// @Param	body	body 	models.Tidb	true	"body for tidb content"
// @Success 200
// @Failure 403 body is empty
// @router / [post]
func (dc *TidbController) Post() {
	db := models.NewTidb()
	b := dc.Ctx.Input.RequestBody
	if len(b) < 1 {
		dc.CustomAbort(403, "body is empty")
	}
	if err := json.Unmarshal(b, db); err != nil {
		dc.CustomAbort(400, fmt.Sprintf("Parse body error: %v", err))
	}
	err := db.Create()
	if err != nil {
		logs.Error("Create tidb-%s error: %v", db.Cell, err)
		dc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("Create tidb-%s error: %v", db.Cell, err))
	}
	// 权限初始化
	if err := models.InitTidb(db.Cell); err != nil {
		logs.Error("Init tidb-%s privilege error: %v", db.Cell, err)
		dc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("Init tidb-%s error: %v", db.Cell, err))
	}
	dc.Data["json"] = db.Cell
	dc.ServeJSON()
}

// Delete 删除tidb服务
// @Title Delete
// @Description delete the tidb service
// @Param	cell	path 	string	true "The cell you want to delete"
// @Success 200 {string} delete success!
// @Failure 403 cell is empty
// @router /:cell [delete]
func (dc *TidbController) Delete() {
	cell := dc.GetString(":cell")
	if len(cell) < 1 {
		dc.CustomAbort(403, "tidb name is nil")
	}
	err := models.EraseTidb(cell)
	if err != nil {
		logs.Error(`Cannt delete tidb-%s: %v`, cell, err)
		dc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf(`Cannt delete tidb-%s: %v`, cell, err))
	}
	dc.Data["json"] = 1
	dc.ServeJSON()
}

// Get 获取tidb数据
// @Title Get
// @Description get tidb by cell
// @Param cell path string true "The cell for tidb name"
// @Success 200 {object} models.Tidb
// @Failure 404 :key not found
// @router /:cell [get]
func (dc *TidbController) Get() {
	cell := dc.GetString(":cell")
	db, err := models.GetTidb(cell)
	if err != nil {
		logs.Error("Cannt get tidb-%s: %v", cell, err)
		dc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("Cannt get tidb-%s: %v", cell, err))
	}
	dc.Data["json"] = *db
	dc.ServeJSON()
}

// Patch 对指定的tidb进行扩容/缩容
// @Title ScaleTidbs
// @Description scale tidbs
// @Param	cell	path	string	true	"The cell for pd name"
// @Param	body	body 	patch	true	"body for patch content"
// @Success 200
// @Failure 403 body is empty
// @router /:cell/scale [patch]
func (dc *TidbController) Patch() {
	cell := dc.GetString(":cell")
	p := patch{}
	if err := json.Unmarshal(dc.Ctx.Input.RequestBody, &p); err != nil {
		dc.CustomAbort(400, fmt.Sprintf("Parse body for patch error: %v", err))
	}
	if err := models.ScaleTidbs(p.Replicas, cell); err != nil {
		logs.Error("Scale tidb-%s error: %v", cell, err)
		dc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("Scale tidb-%s error: %v", cell, err))
	}
	// dc.ServeJSON()
}

// Transfer data to tidb
// @Title Transfer
// @Description Transfer mysql data to tidb
// @Param 	cell 	path 	string	true	"The database name for tidb"
// @Param	body	body 	mysql.Mysql	true	"Body for src mysql"
// @Success 200
// @Failure 403 body is empty
// @router /:cell/transfer [post]
func (dc *TidbController) Transfer() {
	cell := dc.GetString(":cell")
	if len(cell) < 1 {
		dc.CustomAbort(403, "cell is nil")
	}
	src := &mysql.Mysql{}
	b := dc.Ctx.Input.RequestBody
	if len(b) < 1 {
		dc.CustomAbort(403, "Body is empty")
	}
	if err := json.Unmarshal(b, src); err != nil {
		dc.CustomAbort(400, fmt.Sprintf("Parse body error: %v", err))
	}
	if err := models.Transfer(cell, *src); err != nil {
		logs.Error(`Transfer mysql "%s" to tidb error: %v`, cell, err)
		dc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf(`Transfer mysql "%s" to tidb error: %v`, cell, err))
	}
}

// GetEvents get events
// @Title GetEvents
// @Description get all events
// @Param	cell	path	string	true	"The cell for tidb name"
// @Success 200 {object} []models.Event
// @router /:cell/events [get]
func (dc *TidbController) GetEvents() {
	cell := dc.GetString(":cell")
	if len(cell) < 1 {
		dc.CustomAbort(403, "cell is nil")
	}
	es, err := models.GetEventsBy(cell)
	if err != nil {
		logs.Error("get %s events error: %v", cell, err)
		dc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("get %s events error: %v", cell, err))
	}
	dc.Data["json"] = es
	dc.ServeJSON()
}

// Status start/stop/restart tidb server
// @Title status
// @Description start/stop/restart tidb server
// @Param	cell	path	string	true	"The cell for pd name"
// @Param	body	body 	status	true	"The body data type is {'status': int}"
// @Success 200
// @Failure 400 body is empty
// @Failure 403 unsupport operation
// @router /:cell/status [patch]
func (dc *TidbController) Status() {
	cell := dc.GetString(":cell")
	s := status{}
	if err := json.Unmarshal(dc.Ctx.Input.RequestBody, &s); err != nil {
		dc.CustomAbort(400, fmt.Sprintf("Parse body for patch error: %v", err))
	}
	switch s.Status {
	case 0:
		if err := models.Start(cell); err != nil {
			logs.Error("Start tidb %s error: %v", cell, err)
			dc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("Start tidb %s error: %v", cell, err))
		}
	case 1:
		if err := models.Stop(cell, nil); err != nil {
			logs.Error("Stop tidb %s error: %v", cell, err)
			dc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("Stop tidb %s error: %v", cell, err))
		}
	case 2:
		if err := models.Restart(cell); err != nil {
			logs.Error("Restart tidb %s error: %v", cell, err)
			dc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("Restart tidb %s error: %v", cell, err))
		}
	default:
		dc.CustomAbort(403, "unsupport operation")
	}
}

type status struct {
	Status int `json:"status"`
}
