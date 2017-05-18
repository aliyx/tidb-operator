package controllers

import (
	"github.com/astaxie/beego"
)

// Operations about health
type HealthController struct {
	beego.Controller
}

// Get 返回tidb-k8s的健康状况
// @Title Get
// @Description get health status
// @Success 200
// @Failure 404
// @router /
func (hc *HealthController) Get() {
}
