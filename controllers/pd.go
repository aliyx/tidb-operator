package controllers

import (
	"encoding/json"

	"fmt"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/models"
)

// Operations about pd
type PdController struct {
	beego.Controller
}

// Post 创建pd
// @Title CreatePd
// @Description create pds
// @Param	body	body 	models.Pd	true	"body for pd content"
// @Success 200
// @Failure 403 body is empty
// @router / [post]
func (pc *PdController) Post() {
	pd := models.NewPd()
	if err := json.Unmarshal(pc.Ctx.Input.RequestBody, pd); err != nil {
		pc.CustomAbort(400, fmt.Sprintf("Parse body for pd error: %v", err))
	}
	if err := pd.Create(); err != nil {
		logs.Error("Create pd-%s error: %v", pd.Cell, err)
		pc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("Create pd-%s error: %v", pd.Cell, err))
	}
	pc.Data["json"] = pd.Cell
	pc.ServeJSON()
}

// Delete 删除pd
// @Title Delete
// @Description delete the pd service
// @Param	cell	path 	string	true "The cell you want to delete"
// @Success 200 {string} delete success!
// @Failure 403 cell is empty
// @router /:cell [delete]
func (pc *PdController) Delete() {
	cell := pc.GetString(":cell")
	err := models.DeletePd(cell)
	if err != nil {
		logs.Error("Cannt delete pd-%s: %v", cell, err)
		pc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("Cannt delete pd-%s: %v", cell, err))
	}
	pc.Data["json"] = 1
	pc.ServeJSON()
}

// Get 获取pd数据
// @Title Get
// @Description get pd by cell
// @Param	cell	path	string	true	"The cell for pd name"
// @Success 200 {object} models.Pd
// @Failure 404 :key not found
// @router /:cell [get]
func (pc *PdController) Get() {
	cell := pc.GetString(":cell")
	p, err := models.GetPd(cell)
	if err != nil {
		logs.Error("Cannt get pd-%s: %v", cell, err)
		pc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("Cannt get pd-%s: %v", cell, err))
	}
	pc.Data["json"] = *p
	pc.ServeJSON()
}

// GetReplicas 获取pd的replicas
// @Title Get
// @Description get pd replicas
// @Param   tikv   query   int  true	"tikv replicas"
// @Param   tidb   query   int	true	"tidb replicas"
// @Success 200 {int}
// @Failure 403 invalid replicas
// @router /replicas [get]
func (pc *PdController) GetReplicas() {
	tikv, err := pc.GetInt("tikv", 0)
	if err != nil || tikv < 3 {
		pc.CustomAbort(403, fmt.Sprintf("Tikv's replicas can not be less than 3: %d", tikv))
	}
	tidb, err := pc.GetInt("tidb", 0)
	if err != nil || tikv < 1 {
		pc.CustomAbort(403, fmt.Sprintf("Tidb's replicas can not be less than 1: %d", tidb))
	}
	r, err := models.GetPdReplicas(tikv, tidb)
	if err != nil {
		pc.CustomAbort(403, fmt.Sprintf("error: %v", err))
	}
	pc.Data["json"] = r
	pc.ServeJSON()
}

// Patch 对指定的pd进行扩容/缩容
// @Title ScalePds
// @Description scale pds
// @Param	cell	path	string	true	"The cell for pd name"
// @Param	body	body 	patch	true	"body for patch content"
// @Success 200
// @Failure 403 body is empty
// @router /:cell/scale [patch]
func (pc *PdController) Patch() {
	cell := pc.GetString(":cell")
	p := patch{}
	if err := json.Unmarshal(pc.Ctx.Input.RequestBody, &p); err != nil {
		logs.Error("Parse body for patch error: %v", err)
		pc.CustomAbort(400, fmt.Sprintf("Parse body for patch error: %v", err))
	}
	if err := models.ScalePds(p.Replicas, cell); err != nil {
		logs.Error("Scale pd-%s error: %v", cell, err)
		pc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("Scale pd-%s error: %v", cell, err))
	}
	// pc.ServeJSON()
}

// patch 接收所有patch请求的body
type patch struct {
	Replicas int `json:"replicas"`
}
