package pdutil

import (
	"fmt"
	"time"

	"github.com/ffan/tidb-operator/pkg/util/retryutil"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/util/httputil"
	"github.com/tidwall/gjson"
)

const (
	// pdReqTimeout access the request timeout for the pd api service
	pdReqTimeout = 3 * time.Second

	pdAPIV1StoresGet     = "http://%s/pd/api/v1/stores"
	pdAPIV1StoreIDDelete = "http://%s/pd/api/v1/store/%d"
	pdAPIV1StoreIDGet    = "http://%s/pd/api/v1/store/%d"
	pdAPIV1LeaderGet     = "http://%s/pd/api/v1/leader"
	pdAPIV1MembersGet    = "http://%s/pd/api/v1/members"
)

func PdStoresGet(host string) (string, error) {
	bs, err := httputil.Get(fmt.Sprintf(pdAPIV1StoresGet, host), pdReqTimeout)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

func PdStoreIDGet(host string, ID int) (string, error) {
	bs, err := httputil.Get(fmt.Sprintf(pdAPIV1StoreIDGet, host, ID), pdReqTimeout)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

// PdStoreDemote demote store from tikv cluster
func PdStoreDemote(host string, id int) error {
	if err := httputil.Delete(fmt.Sprintf(pdAPIV1StoreIDDelete, host, id), pdReqTimeout); err != nil {
		return err
	}
	logs.Info("Store:%d demoted", id)
	return nil
}

// PdStoreDelete delete store from tikv cluster
func PdStoreDelete(host string, id int) error {
	if err := httputil.Delete(fmt.Sprintf(pdAPIV1StoreIDDelete+"?force", host, id), pdReqTimeout); err != nil {
		return err
	}
	logs.Info("Store:%d deleted", id)
	return nil
}

func PdLeaderGet(host string) (string, error) {
	bs, err := httputil.Get(fmt.Sprintf(pdAPIV1LeaderGet, host), pdReqTimeout)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

func PdMembersGet(host string) (string, error) {
	bs, err := httputil.Get(fmt.Sprintf(pdAPIV1MembersGet, host), pdReqTimeout)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

// RetryGetPdMembers retry get pd member when is electing
func RetryGetPdMembers(host string) (string, error) {
	var (
		b   []byte
		err error
	)
	// default elect time is 3s
	retryutil.Retry(time.Second, 5, func() (bool, error) {
		b, err = httputil.Get(fmt.Sprintf(pdAPIV1MembersGet, host), pdReqTimeout)
		// maybe electing
		if err != nil {
			logs.Warn("could not get members, may be electing: %v", err)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return "", err
	}
	return string(b), err
}

func PdMembersGetName(host string) ([]string, error) {
	s, err := PdMembersGet(host)
	if err != nil {
		return nil, err
	}
	result := gjson.Get(s, "members.#.name")
	if result.Type == gjson.Null {
		return nil, fmt.Errorf("cannt get members")
	}
	var mems []string
	for _, name := range result.Array() {
		mems = append(mems, name.String())
	}
	return mems, nil
}

// PdMemberDelete delete member from pd cluster
func PdMemberDelete(host string, name string) error {
	if err := httputil.Delete(fmt.Sprintf(pdAPIV1MembersGet+"/%s", host, name), pdReqTimeout); err != nil {
		return err
	}
	logs.Info("Pd member %q deleted", name)
	return nil
}
