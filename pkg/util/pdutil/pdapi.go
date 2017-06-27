package pdutil

import (
	"fmt"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/util/httputil"
	"github.com/tidwall/gjson"
)

const (
	// pdReqTimeout access the request timeout for the pd api service
	pdReqTimeout = 3 * time.Second

	pdAPIV1StoresGet     = "http://%s/pd/api/v1/stores"
	pdAPIV1StoreIDDelete = "http://%s/pd/api/v1/store/%d"
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

func PdStoreDelete(host string, id int) error {
	if err := httputil.Delete(fmt.Sprintf(pdAPIV1StoreIDDelete, host, id), pdReqTimeout); err != nil {
		return err
	}
	logs.Warn(`store:%d deleted`, id)
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

func PdMemberDelete(host string, name string) error {
	if err := httputil.Delete(fmt.Sprintf(pdAPIV1MembersGet+"/%s", host, name), pdReqTimeout); err != nil {
		return err
	}
	logs.Warn(`pd:%d deleted`, name)
	return nil
}
