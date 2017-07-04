// @APIVersion 1.0.0
// @Title tidb API
// @Description Tidb-operator creates/configures/manages tidb clusters atop Kubernetes.
// @Contact yangxin45@wanda.cn
package routers

import (
	"github.com/astaxie/beego"
	"github.com/ffan/tidb-operator/operator/controllers"
)

func init() {
	ns := beego.NewNamespace("/tidb/api/v1",
		beego.NSNamespace("/metadata",
			beego.NSInclude(
				&controllers.MetadataController{},
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
