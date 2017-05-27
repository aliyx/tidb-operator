package routers

import (
	"github.com/ffan/tidb-k8s/controllers"

	"github.com/astaxie/beego"
)

func init() {
	ns := beego.NewNamespace("/tidb/api/v1",
		beego.NSNamespace("/metadata",
			beego.NSInclude(
				&controllers.MetadataController{},
			),
		),
		beego.NSNamespace("/tikvs",
			beego.NSInclude(
				&controllers.TikvController{},
			),
		),
		beego.NSNamespace("/tidbs",
			beego.NSInclude(
				&controllers.TidbController{},
			),
		),
		beego.NSNamespace("/users",
			beego.NSInclude(
				&controllers.UserController{},
			),
		),
	)
	beego.AddNamespace(ns)
}
