package garbagecollection

import (
	"os"
	"path"

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
	HostName     string
	Dir          string
	ExcludeFiles []string
}

// Recycling implement PVProvisioner#Recycling
func (hp *HostPathPVProvisioner) Recycling(s *operator.Store) error {
	if s.Node == hp.HostName {
		logs.Info("recycling tikv: %s", s.Name)
		p := path.Join(hp.Dir, s.Name)
		return os.RemoveAll(p)
	}
	return nil
}

// Clean implement PVProvisioner#Clean
func (hp *HostPathPVProvisioner) Clean(all []*operator.Store) error {
	files, err := ioutil.ReadDir(hp.Dir)
	if err != nil {
		return err
	}

	for _, file := range files {
		exist := false
		for _, s := range all {
			if file.Name() == s.Name {
				exist = true
				break
			}
		}

		// fileter excluded files
		if exist == false {
			for _, ef := range hp.ExcludeFiles {
				if file.Name() == ef {
					exist = true
					break
				}
			}
		}
		if !exist {
			p := path.Join(hp.Dir, file.Name())
			logs.Info("delete local file %s", p)
			if err = os.RemoveAll(p); err != nil {
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
