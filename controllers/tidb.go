package controllers

import (
	"encoding/json"

	"fmt"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/models"
	"github.com/ffan/tidb-k8s/models/mysql"
)

var (
	// advertiseIP=beego.BConfig.Listen.HTTPSAddr
	statAPI = "%s:%d/tidb/api/v1/tidbs/%s/status"
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
	errHandler(
		dc.Controller,
		db.Save(),
		fmt.Sprintf("Create tidb %s", db.Cell),
	)
	dc.Data["json"] = db.Cell
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
	errHandler(
		dc.Controller,
		err,
		fmt.Sprintf("Cannt get tidb %s", db.Cell),
	)
	dc.Data["json"] = *db
	dc.ServeJSON()
}

// Patch 对指定的tidb进行扩容/缩容
// @Title ScaleTidbs
// @Description scale tidb
// @Param	cell	path	string	true	"The cell for pd name"
// @Param	body	body 	patch	true	"The body data type is {dbReplica: int, kvReplica: int} for scale content"
// @Success 200
// @Failure 403 body is empty
// @router /:cell/scale [patch]
func (dc *TidbController) Patch() {
	cell := dc.GetString(":cell")
	s := scale{}
	if err := json.Unmarshal(dc.Ctx.Input.RequestBody, &s); err != nil {
		dc.CustomAbort(400, fmt.Sprintf("Parse body for patch error: %v", err))
	}
	errHandler(
		dc.Controller,
		models.Scale(cell, s.KvReplica, s.DbReplica),
		fmt.Sprintf("Scale tidb %s", cell),
	)
}

// Migrate data to tidb
// @Title Migrate
// @Description migrate mysql data to tidb
// @Param   sync	query	string	false       "increment sync"
// @Param 	cell 	path	string	true	"The database name for tidb"
// @Param	body	body	mysql.Mysql	true	"Body for src mysql"
// @Success 200
// @Failure 403 body is empty
// @router /:cell/migrate [post]
func (dc *TidbController) Migrate() {
	cell := dc.GetString(":cell")
	if len(cell) < 1 {
		dc.CustomAbort(403, "cell is nil")
	}
	sync := dc.GetString("sync")
	src := &mysql.Mysql{}
	b := dc.Ctx.Input.RequestBody
	if len(b) < 1 {
		dc.CustomAbort(403, "Body is empty")
	}
	if err := json.Unmarshal(b, src); err != nil {
		dc.CustomAbort(400, fmt.Sprintf("Parse body error: %v", err))
	}
	db, err := models.GetTidb(cell)
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
// @Success 200 {object} []models.Event
// @router /:cell/events [get]
func (dc *TidbController) GetEvents() {
	cell := dc.GetString(":cell")
	if len(cell) < 1 {
		dc.CustomAbort(403, "cell is nil")
	}
	es, err := models.GetEventsBy(cell)
	errHandler(
		dc.Controller,
		err,
		fmt.Sprintf("get %s events", cell),
	)
	dc.Data["json"] = es
	dc.ServeJSON()
}

// Status start/stop/restart tidb server
// @Title status
// @Description start/stop/restart tidb server
// @Param	cell	path	string	true	"The cell for pd name"
// @Param	body	body 	status	true	"The body data type is {'type': string, status': string}"
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
	switch s.Type {
	case "migrate":
		td, err := models.GetTidb(cell)
		errHandler(dc.Controller, err, "Patch tidb status")
		td.UpdateMigrateStat(s.Status, "")
		errHandler(dc.Controller, err, "Patch tidb status")
	default:
		switch s.Status {
		case "start":
			errHandler(
				dc.Controller,
				models.Start(cell),
				fmt.Sprintf("Start tidb %s", cell),
			)
		case "stop":
			errHandler(
				dc.Controller,
				models.Stop(cell, nil),
				fmt.Sprintf("Stop tidb %s", cell),
			)
		case "retart":
			errHandler(
				dc.Controller,
				models.Restart(cell),
				fmt.Sprintf("Restart tidb %s", cell),
			)
		default:
			dc.CustomAbort(403, "unsupport operation")
		}
	}
}

func errHandler(c beego.Controller, err error, msg string) {
	if err == nil {
		return
	}
	logs.Error("%s: %v", msg, err)
	c.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("%s error: %v", msg, err))
}

type status struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	Desc   string `json:"desc"`
}

type scale struct {
	DbReplica int `json:"dbReplica"`
	KvReplica int `json:"kvReplica"`
}
