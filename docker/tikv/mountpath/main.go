package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/nightlyone/lockfile"
)

func main() {
	var hostPath, mount string
	if len(os.Args) >= 2 {
		hostPath = os.Args[1]
	} else {
		hostPath = "/tmp"
	}
	if len(os.Args) >= 3 {
		mount = os.Args[2]
	}
	if len(mount) < 1 {
		defaultPath(hostPath, mount)
		return
	}
	if dir := exist(hostPath, mount); dir != "" {
		fmt.Println(dir)
		return
	}

	lock, err := locker(hostPath)
	if err != nil {
		defaultPath(hostPath, mount)
		return
	}
	if err = lock.TryLock(); err != nil {
		defaultPath(hostPath, mount)
		return
	}
	defer lock.Unlock()

	fis, err := ioutil.ReadDir(hostPath)
	if err != nil {
		defaultPath(hostPath, mount)
		return
	}

	all := []string{}
	for _, f := range fis {
		if !f.IsDir() {
			continue
		}
		if strings.HasPrefix(f.Name(), mount) {
			all = append(all, f.Name())
		}
	}

	count := 0
	var mnt string
	for _, d := range all {
		fis, err = ioutil.ReadDir(fmt.Sprintf("%s/%s", hostPath, d))
		if err != nil {
			defaultPath(hostPath, mount)
		} else {
			cur := len(fis)
			if cur > 0 {
				if count == 0 {
					count = cur
					mnt = d
				} else if cur < count {
					count = cur
					mnt = d
				}
				continue
			} else {
				mnt = d
				break
			}
		}
	}
	defaultPath(hostPath, mnt)
}

func defaultPath(hostPath, mount string) {
	fmt.Println(filepath.Join(hostPath, mount))
}

func locker(hostPath string) (lockfile.Lockfile, error) {
	if hostPath == "/" {
		hostPath = "/tmp"
	}
	return lockfile.New(filepath.Join(hostPath, "tidb.lock"))
}

func exist(hostPath, mount string) string {
	host, err := os.Hostname()
	if err != nil {
		return ""
	}
	dir := filepath.Join(hostPath, mount)
	if strings.LastIndexAny(dir, "/") != (len(dir) - 1) {
		dir = dir + "*"
	}
	files, err := filepath.Glob(filepath.Join(dir, host))
	if err != nil {
		return ""
	}
	if len(files) > 0 {
		return strings.TrimSuffix(files[0], "/"+host)
	}
	return ""
}
