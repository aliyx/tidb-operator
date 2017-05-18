package controllers

import (
	"encoding/json"

	"fmt"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/models"
)

// Operations about metadata
type MetadataController struct {
	beego.Controller
}

// GetAll 获取完整的metadata
// @Title GetAll
// @Description get all metatada
// @Success 200 {object} models.Metadata
// @Failure 404 not find
// @Failure 500 etcd error
// @router / [get]
func (mc *MetadataController) GetAll() {
	md, err := models.GetMetadata()
	if err != nil {
		logs.Error("Cannt get all meatadata: %v", err)
		mc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("%v", err))
	} else {
		mc.Data["json"] = md
	}
	mc.ServeJSON()
}

// PutAll 更新metadata
// @Title Update
// @Description update the metadata
// @Param	body	body	models.Metadata	true	"body for metadata content"
// @Success 200
// @Failure 400 bad request
// @Failure 500 cannt store metadata
// @router / [put]
func (mc *MetadataController) PutAll() {
	md := models.NewMetadata()
	if err := json.Unmarshal(mc.Ctx.Input.RequestBody, md); err != nil {
		mc.CustomAbort(400, fmt.Sprintf("Parse body for metadata error: %v", err))
	}
	if err := md.Update(); err != nil {
		logs.Error("Cannt update all meatadata: %v", err)
		mc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("Cannt update all meatadata: %v", err))
	}
}

// Get 获取metadata子元素
// @Title Get
// @Description get sub metadata by key
// @Param key path string true "The key for metadata property"
// @Success 200 {string}
// @Failure 404 :key not found
// @router /:key [get]
func (mc *MetadataController) Get() {
	key := mc.GetString(":key")
	md, err := models.GetMetadata()
	if err != nil {
		logs.Error("Cannt get meatadata: %v", err)
		mc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("Cannt get meatadata: %v", err))
	}

	var prop interface{}
	switch key {
	case "versions":
		prop = md.Versions
	case "basic":
		prop = md.Units
	case "bpd":
		prop = md.Units.Pd
	case "btikv":
		prop = md.Units.Tikv
	case "btidb":
		prop = md.Units.Tidb
	case "k8s":
		prop = md.K8s
	case "specifications":
		sp, err := models.GetSpecs()
		if err != nil {
			logs.Error("Cannt get specifications: %v", err)
			mc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("Cannt get specifications: %v", err))
		}
		prop = sp
	default:
		mc.CustomAbort(404, fmt.Sprintf("Metadata has no %s property", key))
	}
	mc.Data["json"] = prop
	mc.ServeJSON()
}

// PutSpec 更新spec
// @Title PutSpec
// @Description update the specifications
// @Param	body	body	models.Specifications	true	"body for spec content"
// @Success 200
// @Failure 400 bad request
// @Failure 500 cannt store specifications
// @router /specifications [put]
func (mc *MetadataController) PutSpec() {
	sp := models.NewSpecifications()
	if err := json.Unmarshal(mc.Ctx.Input.RequestBody, sp); err != nil {
		mc.CustomAbort(400, fmt.Sprintf("Parse body for specifications error: %v", err))
	}
	if err := sp.Update(); err != nil {
		logs.Error("Cannt update specifications: %v", err)
		mc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("Cannt update specifications: %v", err))
	}
}
