package controllers

import (
	"encoding/json"
	"fmt"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/models"
)

// Operations about tikv
type TikvController struct {
	beego.Controller
}

// Get 获取tikv数据
// @Title Get
// @Description get tikv by cell
// @Param cell path string true "The cell for tikv name"
// @Success 200 {object} models.Tikv
// @Failure 404 :key not found
// @router /:cell [get]
func (tc *TikvController) Get() {
	cell := tc.GetString(":cell")
	kv, err := models.GetTikv(cell)
	if err != nil {
		logs.Error("Cannt get tikv-%s: %v", cell, err)
		tc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("Cannt get tikv-%s: %v", cell, err))
	}
	tc.Data["json"] = *kv
	tc.ServeJSON()
}

// Patch 对指定的tikv进行扩容/缩容
// @Title ScaleTikvs
// @Description scale tikvs
// @Param	cell	path	string	true	"The cell for pd name"
// @Param	body	body 	patch	true	"body for patch content"
// @Success 200
// @Failure 403 body is empty
// @router /:cell/scale [patch]
func (tc *TikvController) Patch() {
	cell := tc.GetString(":cell")
	p := patch{}
	if err := json.Unmarshal(tc.Ctx.Input.RequestBody, &p); err != nil {
		tc.CustomAbort(400, fmt.Sprintf("Parse body for patch error: %v", err))
	}
	if err := models.ScaleTikvs(p.Replicas, cell); err != nil {
		logs.Error("Scale tikv-%s error: %v", cell, err)
		tc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("Scale tikv-%s error: %v", cell, err))
	}
}
