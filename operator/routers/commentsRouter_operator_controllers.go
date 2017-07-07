package routers

import (
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/context/param"
)

func init() {

	beego.GlobalControllerRouter["github.com/ffan/tidb-operator/operator/controllers:MetadataController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-operator/operator/controllers:MetadataController"],
		beego.ControllerComments{
			Method: "GetAll",
			Router: `/`,
			AllowHTTPMethods: []string{"get"},
			MethodParams: param.Make(),
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-operator/operator/controllers:MetadataController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-operator/operator/controllers:MetadataController"],
		beego.ControllerComments{
			Method: "PutAll",
			Router: `/`,
			AllowHTTPMethods: []string{"put"},
			MethodParams: param.Make(),
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-operator/operator/controllers:MetadataController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-operator/operator/controllers:MetadataController"],
		beego.ControllerComments{
			Method: "Get",
			Router: `/:key`,
			AllowHTTPMethods: []string{"get"},
			MethodParams: param.Make(),
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-operator/operator/controllers:TidbController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-operator/operator/controllers:TidbController"],
		beego.ControllerComments{
			Method: "Post",
			Router: `/`,
			AllowHTTPMethods: []string{"post"},
			MethodParams: param.Make(),
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-operator/operator/controllers:TidbController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-operator/operator/controllers:TidbController"],
		beego.ControllerComments{
			Method: "Delete",
			Router: `/:cell`,
			AllowHTTPMethods: []string{"delete"},
			MethodParams: param.Make(),
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-operator/operator/controllers:TidbController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-operator/operator/controllers:TidbController"],
		beego.ControllerComments{
			Method: "CheckResources",
			Router: `/:user/limit`,
			AllowHTTPMethods: []string{"post"},
			MethodParams: param.Make(),
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-operator/operator/controllers:TidbController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-operator/operator/controllers:TidbController"],
		beego.ControllerComments{
			Method: "Migrate",
			Router: `/:cell/migrate`,
			AllowHTTPMethods: []string{"post"},
			MethodParams: param.Make(),
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-operator/operator/controllers:TidbController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-operator/operator/controllers:TidbController"],
		beego.ControllerComments{
			Method: "Patch",
			Router: `/:cell`,
			AllowHTTPMethods: []string{"patch"},
			MethodParams: param.Make(),
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-operator/operator/controllers:TidbController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-operator/operator/controllers:TidbController"],
		beego.ControllerComments{
			Method: "Get",
			Router: `/:cell`,
			AllowHTTPMethods: []string{"get"},
			MethodParams: param.Make(),
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-operator/operator/controllers:TidbController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-operator/operator/controllers:TidbController"],
		beego.ControllerComments{
			Method: "GetEvents",
			Router: `/:cell/events`,
			AllowHTTPMethods: []string{"get"},
			MethodParams: param.Make(),
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-operator/operator/controllers:UserController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-operator/operator/controllers:UserController"],
		beego.ControllerComments{
			Method: "GetAll",
			Router: `/:user/tidbs`,
			AllowHTTPMethods: []string{"get"},
			MethodParams: param.Make(),
			Params: nil})

}
