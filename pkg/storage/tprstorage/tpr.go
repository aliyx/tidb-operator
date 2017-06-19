package tprstorage

import (
	"context"
	"fmt"

	"errors"

	"github.com/ffan/tidb-k8s/pkg/k8sutil"
	"github.com/ffan/tidb-k8s/pkg/storage"
	"k8s.io/client-go/rest"
)

const (
	// TPRGroup all resources group
	TPRGroup = "tidb.ffan.com"
	// TPRVersion current version is beta
	TPRVersion = "v1beta1"
	// TPRDescription a trp desc
	TPRDescription = "Managed tidb clusters"
)

var (
	errUnsupportededMethod = errors.New("unsupported method")
)

// Storage 实现Impl接口
type Storage struct {
	Namespace string
	Name      string

	tprClient *rest.RESTClient
}

func (s *Storage) listURI(ns string) string {
	return fmt.Sprintf("/apis/%s/%s/namespaces/%s/%ss", TPRGroup, TPRVersion, s.Namespace, s.Name)
}

func (s *Storage) kindPlural() string {
	return s.Name + "s"
}

// Close is part of the storage.Impl interface.
func (s *Storage) Close() error {
	return errUnsupportededMethod
}

// List is part of the storage.Impl interface.
func (s *Storage) List(ctx context.Context) ([]string, error) {
	return nil, nil
}

// ListKey is part of the storage.Impl interface.
func (s *Storage) ListKey(ctx context.Context, prefix string) ([]string, error) {
	return nil, nil
}

// Create is part of the storage.Impl interface.
func (s *Storage) Create(ctx context.Context, key string, contents []byte) (storage.Version, error) {
	return nil, nil
}

// Delete is part of the storage.Impl interface.
func (s *Storage) Delete(ctx context.Context, key string, version storage.Version) error {
	return nil
}

// DeleteAll is part of the storage.Impl interface.
func (s *Storage) DeleteAll(ctx context.Context, key string) error {
	return nil
}

// Update is part of the storage.Impl interface.
func (s *Storage) Update(ctx context.Context, key string, contents []byte, version storage.Version) (storage.Version, error) {
	return nil, nil
}

// Get is part of the storage.Impl interface.
func (s *Storage) Get(ctx context.Context, key string) ([]byte, storage.Version, error) {
	return nil, nil, nil
}

// NewStorage return a new etcdstorage.Storage
func NewStorage(serverAddr, schema, name string) (*Storage, error) {
	cli, err := k8sutil.NewTPRClient(TPRGroup, TPRVersion)
	if err != nil {
		return nil, err
	}
	return &Storage{
		Namespace: schema,
		Name:      name,
		tprClient: cli,
	}, nil
}

func init() {
	storage.RegisterStorageFactory("tpr", func(serverAddr, schema, name string) (storage.Impl, error) {
		return NewStorage(serverAddr, schema, name)
	})
}

var _ storage.Impl = &Storage{} // compile-time interface check
