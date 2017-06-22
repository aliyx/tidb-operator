package storage

import (
	"fmt"
	"net/http"

	"errors"

	"encoding/json"

	"github.com/ffan/tidb-k8s/pkg/k8sutil"
	"github.com/ffan/tidb-k8s/pkg/spec"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/rest"
)

var (
	// ErrCreateEtcdStorage fail to create etcd
	ErrCreateEtcdStorage = errors.New("cannt creat etcd storage")
	// ErrNoNode is returned by functions to specify the requested
	// resource does not exist.
	ErrNoNode = errors.New("node doesn't exist")
	// ErrAlreadyExists resource already exit
	ErrAlreadyExists = errors.New("node already exist")
	// ErrTimeout is returned by functions that wait for a result
	// when the timeout value is reached.
	ErrTimeout = errors.New("deadline exceeded")
	// ErrInterrupted is returned by functions that wait for a result
	// when they are interrupted.
	ErrInterrupted = errors.New("interrupted")
	// ErrBadVersion is returned by an update function that
	// failed to update the data because the version was different
	ErrBadVersion = errors.New("bad node version")
)

// Storage implement Impl interface
type Storage struct {
	Namespace string
	Name      string

	tprClient *rest.RESTClient
}

func (s *Storage) listURI() string {
	return fmt.Sprintf("/apis/%s/%s/namespaces/%s/%ss/", spec.TPRGroup, spec.TPRVersion, s.Namespace, s.Name)
}

func (s *Storage) kindPlural() string {
	return s.Name + "s"
}

// List query all.
// FIXME: prefix
func (s *Storage) List(prefix string, v interface{}) error {
	b, err := s.tprClient.Get().
		RequestURI(s.listURI()).
		// FieldsSelectorParam(fields.Set{"metadata.name": "test"}.AsSelector()).
		DoRaw()
	if err != nil {
		return err
	}
	if err = json.Unmarshal(b, v); err != nil {
		return err
	}
	return nil
}

// Create is part of the storage.Impl interface.
func (s *Storage) Create(v interface{}) error {
	err := s.tprClient.Post().
		Resource(s.kindPlural()).
		Namespace(s.Namespace).
		Body(v).
		Do().Error()
	if apierrors.IsAlreadyExists(err) {
		return ErrAlreadyExists
	}
	return err
}

// Delete is part of the storage.Impl interface.
func (s *Storage) Delete(key string) error {
	err := s.tprClient.Delete().
		Resource(s.kindPlural()).
		Namespace(s.Namespace).
		Name(key).
		Do().Error()
	if apierrors.IsNotFound(err) {
		return ErrNoNode
	}
	return err
}

// DeleteAll is part of the storage.Impl interface.
func (s *Storage) DeleteAll() error {
	return s.tprClient.Delete().
		Resource(s.kindPlural()).
		Namespace(s.Namespace).
		Do().Error()
}

// Update is part of the storage.Impl interface.
func (s *Storage) Update(key string, v interface{}) error {
	for {
		var statusCode int

		err := s.tprClient.Put().
			Resource(s.kindPlural()).
			Namespace(s.Namespace).
			Name(key).
			Body(v).
			Do().StatusCode(&statusCode).Error()

		if statusCode == http.StatusConflict {
			continue
		}

		return err
	}
}

// Get is part of the storage.Impl interface.
func (s *Storage) Get(key string, v interface{}) error {
	b, err := s.tprClient.Get().
		Resource(s.kindPlural()).
		Namespace(s.Namespace).
		Name(key).
		DoRaw()

	if err != nil {
		if apierrors.IsNotFound(err) {
			return ErrNoNode
		}
		return err
	}
	if err = json.Unmarshal(b, v); err != nil {
		return err
	}
	return nil
}

// NewStorage return a new etcdstorage.Storage
func NewStorage(namespace, name string) (*Storage, error) {
	cli, err := k8sutil.NewTPRClient(spec.TPRGroup, spec.TPRVersion)
	if err != nil {
		return nil, err
	}
	if err = k8sutil.CreateTPR(name); err != nil {
		return nil, err
	}
	return &Storage{
		Namespace: namespace,
		Name:      name,
		tprClient: cli,
	}, nil
}
