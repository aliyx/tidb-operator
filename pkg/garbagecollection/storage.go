package garbagecollection

import (
	"os"
	"path"
)

// PVProvisioner persistent volumes provisioner
type PVProvisioner interface {
	Recycling(p string) error
}

// HostPathPVProvisioner local storage
type HostPathPVProvisioner struct {
	Dir string
}

// Recycling tikv host resource
func (hp *HostPathPVProvisioner) Recycling(id string) error {
	p := path.Join(hp.Dir, id)
	return os.RemoveAll(p)
}

// NilPVProvisioner pod's storage is pod itself
type NilPVProvisioner struct{}

// Recycling tikv host resource
func (n *NilPVProvisioner) Recycling(id string) error {
	return nil
}
