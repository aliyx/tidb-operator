package routers

import (
	"github.com/astaxie/beego"
)

func init() {

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:HealthController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:HealthController"],
		beego.ControllerComments{
			Method:           "Get",
			Router:           `/`,
			AllowHTTPMethods: []string{"get"},
			Params:           nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:MetadataController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:MetadataController"],
		beego.ControllerComments{
			Method:           "GetAll",
			Router:           `/`,
			AllowHTTPMethods: []string{"get"},
			Params:           nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:MetadataController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:MetadataController"],
		beego.ControllerComments{
			Method:           "PutAll",
			Router:           `/`,
			AllowHTTPMethods: []string{"put"},
			Params:           nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:MetadataController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:MetadataController"],
		beego.ControllerComments{
			Method:           "Get",
			Router:           `/:key`,
			AllowHTTPMethods: []string{"get"},
			Params:           nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:MetadataController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:MetadataController"],
		beego.ControllerComments{
			Method:           "PutSpec",
			Router:           `/specifications`,
			AllowHTTPMethods: []string{"put"},
			Params:           nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:PdController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:PdController"],
		beego.ControllerComments{
			Method:           "Post",
			Router:           `/`,
			AllowHTTPMethods: []string{"post"},
			Params:           nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:PdController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:PdController"],
		beego.ControllerComments{
			Method:           "Delete",
			Router:           `/:cell`,
			AllowHTTPMethods: []string{"delete"},
			Params:           nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:PdController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:PdController"],
		beego.ControllerComments{
			Method:           "Get",
			Router:           `/:cell`,
			AllowHTTPMethods: []string{"get"},
			Params:           nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:PdController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:PdController"],
		beego.ControllerComments{
			Method:           "GetReplicas",
			Router:           `/replicas`,
			AllowHTTPMethods: []string{"get"},
			Params:           nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:PdController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:PdController"],
		beego.ControllerComments{
			Method:           "Patch",
			Router:           `/:cell/scale`,
			AllowHTTPMethods: []string{"patch"},
			Params:           nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TidbController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TidbController"],
		beego.ControllerComments{
			Method:           "Post",
			Router:           `/`,
			AllowHTTPMethods: []string{"post"},
			Params:           nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TidbController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TidbController"],
		beego.ControllerComments{
			Method:           "Delete",
			Router:           `/:cell`,
			AllowHTTPMethods: []string{"delete"},
			Params:           nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TidbController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TidbController"],
		beego.ControllerComments{
			Method:           "Get",
			Router:           `/:cell`,
			AllowHTTPMethods: []string{"get"},
			Params:           nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TidbController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TidbController"],
		beego.ControllerComments{
			Method:           "Patch",
			Router:           `/:cell/scale`,
			AllowHTTPMethods: []string{"patch"},
			Params:           nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TikvController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TikvController"],
		beego.ControllerComments{
			Method:           "Post",
			Router:           `/`,
			AllowHTTPMethods: []string{"post"},
			Params:           nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TikvController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TikvController"],
		beego.ControllerComments{
			Method:           "Delete",
			Router:           `/:cell`,
			AllowHTTPMethods: []string{"delete"},
			Params:           nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TikvController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TikvController"],
		beego.ControllerComments{
			Method:           "Get",
			Router:           `/:cell`,
			AllowHTTPMethods: []string{"get"},
			Params:           nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TikvController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:TikvController"],
		beego.ControllerComments{
			Method:           "Patch",
			Router:           `/:cell/scale`,
			AllowHTTPMethods: []string{"patch"},
			Params:           nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:UserController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:UserController"],
		beego.ControllerComments{
			Method:           "Post",
			Router:           `/`,
			AllowHTTPMethods: []string{"post"},
			Params:           nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:UserController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:UserController"],
		beego.ControllerComments{
			Method:           "Delete",
			Router:           `/:user/tidbs/:cell`,
			AllowHTTPMethods: []string{"delete"},
			Params:           nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:UserController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:UserController"],
		beego.ControllerComments{
			Method:           "Get",
			Router:           `/:user/tidbs/:cell`,
			AllowHTTPMethods: []string{"get"},
			Params:           nil})

	beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:UserController"] = append(beego.GlobalControllerRouter["github.com/ffan/tidb-k8s/controllers:UserController"],
		beego.ControllerComments{
			Method:           "GetAll",
			Router:           `/:user/tidbs`,
			AllowHTTPMethods: []string{"get"},
			Params:           nil})

}
