package etcdstorage

import (
	"context"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/ffan/tidb-k8s/models"
)

// etcdClient 访问etcd接口
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
	ec *etcdClient
}

// Close is part of the models.Impl interface.
func (s *Storage) Close() {
	s.ec.cli.Close()
}

// ListDir is part of the models.Impl interface.
func (s *Storage) ListDir(ctx context.Context, dirPath string) ([]string, error) {
	nodePath := dirPath + "/"
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
		return nil, models.ErrNoNode
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

// ListKey is part of the models.Impl interface.
func (s *Storage) ListKey(ctx context.Context, prefix string) ([]string, error) {
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
		return nil, models.ErrNoNode
	}

	nodePath := prefix
	i := strings.LastIndex(prefix, "/")
	if i >= 0 {
		nodePath = prefix[i+1:]
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

// Create is part of the models.Impl interface.
func (s *Storage) Create(ctx context.Context, path string, contents []byte) (models.Version, error) {
	resp, err := s.ec.cli.Put(ctx, path, string(contents))
	if err != nil {
		return nil, convertError(err)
	}
	return EtcdVersion(resp.Header.Revision), nil
}

// Delete is part of the models.Impl interface.
func (s *Storage) Delete(ctx context.Context, path string, version models.Version) error {
	if version != nil {
		// We have to do a transaction. This means: if the
		// node revision is what we expect, delete it,
		// otherwise get the file. If the transaction doesn't
		// succeed, we also ask for the value of the
		// node. That way we'll know if it failed because it
		// didn't exist, or because the version was wrong.
		txnresp, err := s.ec.cli.Txn(ctx).
			If(clientv3.Compare(clientv3.ModRevision(path), "=", int64(version.(EtcdVersion)))).
			Then(clientv3.OpDelete(path)).
			Else(clientv3.OpGet(path)).
			Commit()
		if err != nil {
			return convertError(err)
		}
		if !txnresp.Succeeded {
			if len(txnresp.Responses) > 0 {
				if len(txnresp.Responses[0].GetResponseRange().Kvs) > 0 {
					return models.ErrBadVersion
				}
			}
			return models.ErrNoNode
		}
		return nil
	}

	// This is just a regular unconditional Delete here.
	resp, err := s.ec.cli.Delete(ctx, path)
	if err != nil {
		return convertError(err)
	}
	if resp.Deleted != 1 {
		return models.ErrNoNode
	}
	return nil
}

// DeleteAll is part of the models.Impl interface.
func (s *Storage) DeleteAll(ctx context.Context, path string) error {
	// This is just a regular unconditional Delete here.
	_, err := s.ec.cli.Delete(ctx, path, clientv3.WithPrefix())
	if err != nil {
		return convertError(err)
	}
	return nil
}

// Update is part of the models.Impl interface.
func (s *Storage) Update(ctx context.Context, path string, contents []byte, version models.Version) (models.Version, error) {
	if version != nil {
		// We have to do a transaction. This means: if the
		// current file revision is what we expect, save it.
		txnresp, err := s.ec.cli.Txn(ctx).
			If(clientv3.Compare(clientv3.ModRevision(path), "=", int64(version.(EtcdVersion)))).
			Then(clientv3.OpPut(path, string(contents))).
			Commit()
		if err != nil {
			return nil, convertError(err)
		}
		if !txnresp.Succeeded {
			return nil, models.ErrBadVersion
		}
		return EtcdVersion(txnresp.Header.Revision), nil
	}

	// No version specified. We can use a simple unconditional Put.
	resp, err := s.ec.cli.Put(ctx, path, string(contents))
	if err != nil {
		return nil, convertError(err)
	}
	return EtcdVersion(resp.Header.Revision), nil
}

// Get is part of the models.Impl interface.
func (s *Storage) Get(ctx context.Context, path string) ([]byte, models.Version, error) {
	resp, err := s.ec.cli.Get(ctx, path)
	if err != nil {
		return nil, nil, convertError(err)
	}
	if len(resp.Kvs) != 1 {
		return nil, nil, models.ErrNoNode
	}
	return resp.Kvs[0].Value, EtcdVersion(resp.Header.Revision), nil
}

// NewStorage return a new etcdstorage.Storage
func NewStorage(serverAddr string) (*Storage, error) {
	ec, err := newEtcdClient(serverAddr)
	if err != nil {
		return nil, err
	}
	return &Storage{
		ec: ec,
	}, nil
}

func init() {
	models.RegisterStorageFactory("etcd", func(serverAddr string) (models.Impl, error) {
		return NewStorage(serverAddr)
	})
}

var _ models.Impl = &Storage{} // compile-time interface check
