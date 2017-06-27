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
	errHandler(
		mc.Controller,
		md.Update(),
		fmt.Sprintf("update all meatadata"),
	)
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
		prop = md.Spec.Versions
	case "pd":
		prop = md.Spec.Pd
	case "tikv":
		prop = md.Spec.Tikv
	case "tidb":
		prop = md.Spec.Tidb
	case "k8s":
		prop = md.Spec.K8s
	case "specifications":
		prop = md.Spec.Specifications
	default:
		mc.CustomAbort(404, fmt.Sprintf("Metadata has no %s property", key))
	}
	mc.Data["json"] = prop
	mc.ServeJSON()
}
