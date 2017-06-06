package models

import (
	"os"
	"strings"

	"fmt"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/models/utils"
)

var (
	onInitHooks utils.Hooks
)

// Init model初始化
func Init() {
	defer func() {
		if err := recover(); err != nil {
			logs.Critical("Init tidb-k8s error: %v", err)
			os.Exit(1)
		}
	}()
	onInitHooks.Add(configInit)
	onInitHooks.Add(metaInit)
	onInitHooks.Add(specInit)
	onInitHooks.Add(tidbInit)
	onInitHooks.Add(userInit)
	onInitHooks.Add(eventInit)
	onInitHooks.Fire()

	initK8sNamespace()
}

func initK8sNamespace() {
	md, err := GetMetadata()
	if err != nil {
		panic(fmt.Errorf("get metadata error: %v", err))
	}
	r := strings.NewReplacer("{{namespace}}", md.K8s.Namespace)
	s := r.Replace(tidbNamespaceYaml)
	if err := createNamespace(s); err != nil {
		if err == utils.ErrAlreadyExists {
			logs.Warn(`Namespace "%s" already exists`, md.K8s.Namespace)
		} else {
			logs.Critical("Init k8s namespace error: %v", err)
			panic(err)
		}
	}
}
