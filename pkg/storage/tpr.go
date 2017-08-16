package storage

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"errors"

	"encoding/json"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/spec"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
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
	// ErrConflict resource version conflict
	ErrConflict = errors.New("conflict")
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
func (s *Storage) List(v interface{}) error {
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

// Create create a new resource.
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

// Delete delete a resource.
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

// DeleteAll delete all tpr.
func (s *Storage) DeleteAll() error {
	return s.tprClient.Delete().
		Resource(s.kindPlural()).
		Namespace(s.Namespace).
		Do().Error()
}

// Update update a tpr.
func (s *Storage) Update(key string, v interface{}) error {
	var statusCode int
	err := s.tprClient.Put().
		Resource(s.kindPlural()).
		Namespace(s.Namespace).
		Name(key).
		Body(v).
		Do().StatusCode(&statusCode).Error()

	if statusCode == http.StatusConflict {
		return ErrConflict
	}
	return err
}

// RetryUpdate retry max 5 time to update a tpr.
func (s *Storage) RetryUpdate(key string, v interface{}) error {
	retryCount := 0
	for {
		r := spec.Resource{}
		if err := s.Get(key, &r); err != nil {
			return err
		}

		// set resourceVersion
		rv := reflect.ValueOf(r).FieldByName("Metadata").FieldByName("ResourceVersion").String()
		reflect.ValueOf(v).Elem().FieldByName("Metadata").FieldByName("ResourceVersion").SetString(rv)

		var statusCode int
		err := s.tprClient.Put().
			Resource(s.kindPlural()).
			Namespace(s.Namespace).
			Name(key).
			Body(v).
			Do().StatusCode(&statusCode).Error()
		if statusCode == http.StatusConflict {
			if retryCount > 5 {
				logs.Warn("retry update trp %s %d times", key, retryCount)
			}
			retryCount++
			continue
		}

		return err
	}
}

// Get get a tpr.
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

// NewStorage return a new storage.Storage
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
		Name:      strings.ToLower(name),
		tprClient: cli,
	}, nil
}
