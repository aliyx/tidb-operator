package servenv

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Root returns the root directory of the current project
func Root() string {
	return fmt.Sprintf("%s/src/github.com/ffan/tidb-operator", GOPATH())
}

// GOPATH gets the current GOPATH
func GOPATH() string {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = defaultGOPATH()
	}
	return gopath
}

func defaultGOPATH() string {
	env := "HOME"
	if runtime.GOOS == "windows" {
		env = "USERPROFILE"
	} else if runtime.GOOS == "plan9" {
		env = "home"
	}
	if home := os.Getenv(env); home != "" {
		def := filepath.Join(home, "go")
		if filepath.Clean(def) == filepath.Clean(runtime.GOROOT()) {
			// Don't set the default GOPATH to GOROOT,
			// as that will trigger warnings from the go tool.
			return ""
		}
		return def
	}
	return ""
}
