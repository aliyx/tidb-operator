package test

var (
	createJson = `{"pd":{"version":"rc3"},"tikv":{"replicas":3,"version":"rc3"},"tidb":{"replicas":1,"version":"rc3"},"owner":{"userId":"6","userName":"yangxin45","desc":""},"schema":{"name":"xinyang1","user":"xinyang1","password":"xinyang1"},"status":{"phase":0}}`

	start = `[{"op":"replace","path":"/operator","value":"start"}]`
)
