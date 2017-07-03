package garbagecollection

import (
	"os"
	"path"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/operator"
)

// PVProvisioner persistent volumes provisioner
type PVProvisioner interface {
	Recycling(s *operator.Store) error
}

// HostPathPVProvisioner local storage
type HostPathPVProvisioner struct {
	HostName string
	Dir      string
}

// Recycling tikv host resource
func (hp *HostPathPVProvisioner) Recycling(s *operator.Store) error {
	if s.Node == hp.HostName {
		logs.Info("recycling tikv: %s", s.Name)
		p := path.Join(hp.Dir, s.Name)
		return os.RemoveAll(p)
	}
	return nil
}

// NilPVProvisioner pod's storage is pod itself
type NilPVProvisioner struct{}

// Recycling tikv host resource
func (n *NilPVProvisioner) Recycling(s *operator.Store) error {
	return nil
}
