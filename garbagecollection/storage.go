package garbagecollection

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"

	"io/ioutil"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/operator"
)

var (
	found = errors.New("found")
)

// PVProvisioner persistent volumes provisioner
type PVProvisioner interface {
	Recycling(s *operator.Store) error
	// Clean clear local resource
	Clean(all []*operator.Store) error
}

// HostPathPVProvisioner local storage
type HostPathPVProvisioner struct {
	Node         string
	HostPath     string
	Mount        string
	ExcludeFiles []string
}

// Recycling implement PVProvisioner#Recycling
func (hp *HostPathPVProvisioner) Recycling(s *operator.Store) error {
	if s.Node != hp.Node {
		return nil
	}
	if len(s.Name) < 1 {
		return nil
	}

	logs.Info("start recycling tikv: %s", s.Name)
	err := filepath.Walk(hp.HostPath, func(path string, info os.FileInfo, err error) error {
		// end
		if err != nil {
			return err
		}
		// continue
		if !info.IsDir() {
			return nil
		}

		if path == hp.HostPath {
			return nil
		}
		// end if found
		if strings.HasSuffix(path, s.Name) {
			os.RemoveAll(path)
			logs.Info("%s is deleted", path)
			return found
		}
		// Only handle with the 'mount' suffix directory
		if strings.Contains(path, "/"+hp.Mount) {
			return nil
		}
		return filepath.SkipDir
	})
	if err != found {
		return err
	}
	return nil
}

// Clean implement PVProvisioner#Clean
func (hp *HostPathPVProvisioner) Clean(all []*operator.Store) error {
	var tikvs []string
	mnts, err := ioutil.ReadDir(hp.HostPath)
	if err != nil {
		return err
	}
	if len(hp.Mount) > 0 {
		for _, p := range mnts {
			if p.IsDir() && strings.HasPrefix(p.Name(), hp.Mount) {
				fs, err := ioutil.ReadDir(path.Join(hp.HostPath, p.Name()))
				if err != nil {
					return err
				}
				for _, f := range fs {
					tikvs = append(tikvs, path.Join(hp.HostPath, p.Name(), f.Name()))
				}
			}
		}
	} else {
		for _, f := range mnts {
			tikvs = append(tikvs, path.Join(hp.HostPath, f.Name()))
		}
	}

	for _, file := range tikvs {
		exist := false
		for _, s := range all {
			if strings.HasSuffix(file, s.Name) {
				exist = true
				break
			}
		}

		// fileter excluded files
		if exist == false {
			for _, ef := range hp.ExcludeFiles {
				if strings.HasSuffix(file, ef) {
					exist = true
					break
				}
			}
		}
		if !exist {
			logs.Info("file %s is deleted", file)
			if err = os.RemoveAll(file); err != nil {
				return err
			}
		}
	}
	return nil
}

// NilPVProvisioner pod's storage is pod itself
type NilPVProvisioner struct{}

// Recycling tikv host resource
func (n *NilPVProvisioner) Recycling(s *operator.Store) error {
	return nil
}

func (n *NilPVProvisioner) Clean(all []*operator.Store) error {
	return nil
}
