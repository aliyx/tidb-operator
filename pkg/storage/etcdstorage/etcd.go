package etcdstorage

import (
	"context"
	"strings"
	"time"

	"path"

	"github.com/coreos/etcd/clientv3"
	"github.com/ffan/tidb-k8s/pkg/storage"
)

const (
	// RootPath etcd root path
	RootPath = "/tk"
)

type etcdClient struct {
	address string
	cli     *clientv3.Client
}

func newEtcdClient(serverAddr string) (*etcdClient, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   strings.Split(serverAddr, ","),
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, err
	}

	return &etcdClient{
		address: serverAddr,
		cli:     cli,
	}, nil
}

// Storage 实现Impl接口
type Storage struct {
	Schema string
	Name   string

	ec *etcdClient
}

// Close is part of the storage.Impl interface.
func (s *Storage) Close() error {
	return s.ec.cli.Close()
}

// List is part of the storage.Impl interface.
func (s *Storage) List(ctx context.Context) ([]string, error) {
	nodePath := path.Join(s.Schema, s.Name) + "/"
	resp, err := s.ec.cli.Get(ctx, nodePath,
		clientv3.WithPrefix(),
		clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend),
		clientv3.WithKeysOnly())
	if err != nil {
		return nil, convertError(err)
	}
	if len(resp.Kvs) == 0 {
		// No key starts with this prefix, means the directory
		// doesn't exist.
		return nil, storage.ErrNoNode
	}

	prefixLen := len(nodePath)
	var result []string
	for _, ev := range resp.Kvs {
		p := string(ev.Key)

		// Remove the prefix, base path.
		if !strings.HasPrefix(p, nodePath) {
			return nil, ErrBadResponse
		}
		p = p[prefixLen:]

		// Keep only the part until the first '/'.
		if i := strings.Index(p, "/"); i >= 0 {
			p = p[:i]
		}

		// Remove duplicates, add to list.
		if len(result) == 0 || result[len(result)-1] != p {
			result = append(result, p)
		}
	}

	return result, nil
}

// ListKey is part of the storage.Impl interface.
func (s *Storage) ListKey(ctx context.Context, prefix string) ([]string, error) {
	prefix = path.Join(s.Schema, s.Name, prefix)
	resp, err := s.ec.cli.Get(ctx, prefix,
		clientv3.WithPrefix(),
		clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend),
		clientv3.WithKeysOnly())
	if err != nil {
		return nil, convertError(err)
	}
	if len(resp.Kvs) == 0 {
		// No key starts with this prefix, means the directory
		// doesn't exist.
		return nil, storage.ErrNoNode
	}

	nodePath := prefix
	i := strings.LastIndex(prefix, "/")
	if i >= 0 {
		nodePath = prefix[:i+1]
	}
	prefixLen := len(nodePath)
	var result []string
	for _, ev := range resp.Kvs {
		p := string(ev.Key)
		// Remove the prefix, base path.
		if !strings.HasPrefix(p, nodePath) {
			return nil, ErrBadResponse
		}
		p = p[prefixLen:]

		// Keep only the part until the first '/'.
		if i := strings.Index(p, "/"); i >= 0 {
			p = p[:i]
		}

		// Remove duplicates, add to list.
		if len(result) == 0 || result[len(result)-1] != p {
			result = append(result, p)
		}
	}

	return result, nil
}

// Create is part of the storage.Impl interface.
func (s *Storage) Create(ctx context.Context, key string, contents []byte) (storage.Version, error) {
	key = path.Join(s.Schema, s.Name, key)
	resp, err := s.ec.cli.Put(ctx, key, string(contents))
	if err != nil {
		return nil, convertError(err)
	}
	return EtcdVersion(resp.Header.Revision), nil
}

// Delete is part of the storage.Impl interface.
func (s *Storage) Delete(ctx context.Context, key string, version storage.Version) error {
	key = path.Join(s.Schema, s.Name, key)
	if version != nil {
		// We have to do a transaction. This means: if the
		// node revision is what we expect, delete it,
		// otherwise get the file. If the transaction doesn't
		// succeed, we also ask for the value of the
		// node. That way we'll know if it failed because it
		// didn't exist, or because the version was wrong.
		txnresp, err := s.ec.cli.Txn(ctx).
			If(clientv3.Compare(clientv3.ModRevision(key), "=", int64(version.(EtcdVersion)))).
			Then(clientv3.OpDelete(key)).
			Else(clientv3.OpGet(key)).
			Commit()
		if err != nil {
			return convertError(err)
		}
		if !txnresp.Succeeded {
			if len(txnresp.Responses) > 0 {
				if len(txnresp.Responses[0].GetResponseRange().Kvs) > 0 {
					return storage.ErrBadVersion
				}
			}
			return storage.ErrNoNode
		}
		return nil
	}

	// This is just a regular unconditional Delete here.
	resp, err := s.ec.cli.Delete(ctx, key)
	if err != nil {
		return convertError(err)
	}
	if resp.Deleted != 1 {
		return storage.ErrNoNode
	}
	return nil
}

// DeleteAll is part of the storage.Impl interface.
func (s *Storage) DeleteAll(ctx context.Context, key string) error {
	key = path.Join(s.Schema, s.Name, key)
	// This is just a regular unconditional Delete here.
	_, err := s.ec.cli.Delete(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return convertError(err)
	}
	return nil
}

// Update is part of the storage.Impl interface.
func (s *Storage) Update(ctx context.Context, key string, contents []byte, version storage.Version) (storage.Version, error) {
	key = path.Join(s.Schema, s.Name, key)
	if version != nil {
		// We have to do a transaction. This means: if the
		// current file revision is what we expect, save it.
		txnresp, err := s.ec.cli.Txn(ctx).
			If(clientv3.Compare(clientv3.ModRevision(key), "=", int64(version.(EtcdVersion)))).
			Then(clientv3.OpPut(key, string(contents))).
			Commit()
		if err != nil {
			return nil, convertError(err)
		}
		if !txnresp.Succeeded {
			return nil, storage.ErrBadVersion
		}
		return EtcdVersion(txnresp.Header.Revision), nil
	}

	// No version specified. We can use a simple unconditional Put.
	resp, err := s.ec.cli.Put(ctx, key, string(contents))
	if err != nil {
		return nil, convertError(err)
	}
	return EtcdVersion(resp.Header.Revision), nil
}

// Get is part of the storage.Impl interface.
func (s *Storage) Get(ctx context.Context, key string) ([]byte, storage.Version, error) {
	key = path.Join(s.Schema, s.Name, key)
	resp, err := s.ec.cli.Get(ctx, key)
	if err != nil {
		return nil, nil, convertError(err)
	}
	if len(resp.Kvs) != 1 {
		return nil, nil, storage.ErrNoNode
	}
	return resp.Kvs[0].Value, EtcdVersion(resp.Header.Revision), nil
}

// NewStorage return a new etcdstorage.Storage
func NewStorage(serverAddr, name string) (*Storage, error) {
	ec, err := newEtcdClient(serverAddr)
	if err != nil {
		return nil, err
	}
	return &Storage{
		Schema: RootPath,
		Name:   name,
		ec:     ec,
	}, nil
}

func init() {
	storage.RegisterStorageFactory("etcd", func(serverAddr, name string) (storage.Impl, error) {
		return NewStorage(serverAddr, name)
	})
}

var _ storage.Impl = &Storage{} // compile-time interface check
