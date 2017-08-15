package garbagecollection

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"io/ioutil"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/operator"
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
	dir := hp.HostPath
	if len(hp.Mount) > 0 {
		dir += (hp.Mount + "*")
	}
	files, err := filepath.Glob(filepath.Join(dir, s.Name))
	if err != nil {
		return err
	}
	for _, f := range files {
		logs.Info("%s is deleted", f)
		if err = os.RemoveAll(f); err != nil {
			return err
		}
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
