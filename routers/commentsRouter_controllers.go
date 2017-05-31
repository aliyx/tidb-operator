package routers

import (
	"github.com/astaxie/beego"
)

func init() {

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:MetadataController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:MetadataController"],
		beego.ControllerComments{
			Method: "GetAll",
			Router: `/`,
			AllowHTTPMethods: []string{"get"},
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:MetadataController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:MetadataController"],
		beego.ControllerComments{
			Method: "PutAll",
			Router: `/`,
			AllowHTTPMethods: []string{"put"},
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:MetadataController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:MetadataController"],
		beego.ControllerComments{
			Method: "Get",
			Router: `/:key`,
			AllowHTTPMethods: []string{"get"},
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:MetadataController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:MetadataController"],
		beego.ControllerComments{
			Method: "PutSpec",
			Router: `/specifications`,
			AllowHTTPMethods: []string{"put"},
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TidbController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TidbController"],
		beego.ControllerComments{
			Method: "Post",
			Router: `/`,
			AllowHTTPMethods: []string{"post"},
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TidbController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TidbController"],
		beego.ControllerComments{
			Method: "Get",
			Router: `/:cell`,
			AllowHTTPMethods: []string{"get"},
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TidbController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TidbController"],
		beego.ControllerComments{
			Method: "Patch",
			Router: `/:cell/scale`,
			AllowHTTPMethods: []string{"patch"},
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TidbController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TidbController"],
		beego.ControllerComments{
			Method: "Migrate",
			Router: `/:cell/migrate`,
			AllowHTTPMethods: []string{"post"},
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TidbController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TidbController"],
		beego.ControllerComments{
			Method: "GetEvents",
			Router: `/:cell/events`,
			AllowHTTPMethods: []string{"get"},
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TidbController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TidbController"],
		beego.ControllerComments{
			Method: "Status",
			Router: `/:cell/status`,
			AllowHTTPMethods: []string{"patch"},
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:UserController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:UserController"],
		beego.ControllerComments{
			Method: "Post",
			Router: `/`,
			AllowHTTPMethods: []string{"post"},
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:UserController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:UserController"],
		beego.ControllerComments{
			Method: "Delete",
			Router: `/:user/tidbs/:cell`,
			AllowHTTPMethods: []string{"delete"},
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:UserController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:UserController"],
		beego.ControllerComments{
			Method: "Get",
			Router: `/:user/tidbs/:cell`,
			AllowHTTPMethods: []string{"get"},
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:UserController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:UserController"],
		beego.ControllerComments{
			Method: "GetAll",
			Router: `/:user/tidbs`,
			AllowHTTPMethods: []string{"get"},
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:UserController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:UserController"],
		beego.ControllerComments{
			Method: "CheckResources",
			Router: `/:user/limit`,
			AllowHTTPMethods: []string{"post"},
			Params: nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:UserController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:UserController"],
		beego.ControllerComments{
			Method: "Status",
			Router: `/:user/tidbs/:cell/status`,
			AllowHTTPMethods: []string{"patch"},
			Params: nil})

}
