package controllers

import (
	"fmt"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/models"
)

// Operations about all tidbs
type UserController struct {
	beego.Controller
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
		logs.Error("Cannt get %s tidbs: %v", user, err)
		uc.CustomAbort(err2httpStatuscode(err), fmt.Sprintf("Cannt get %s tidbs: %v", user, err))
	}
	uc.Data["json"] = Dbs{len(dbs), dbs}
	uc.ServeJSON()
}

// Dbs db array
type Dbs struct {
	Total int         `json:"total"`
	Tidbs []models.Db `json:"tidbs"`
}
