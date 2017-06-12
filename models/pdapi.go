package models

import (
	"fmt"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/pkg/httputil"
	"github.com/tidwall/gjson"
)

var (
	pdAPIV1StoresGet     = "http://%s/pd/api/v1/stores"
	pdAPIV1StoreIDDelete = "http://%s/pd/api/v1/store/%d"
	pdAPIV1LeaderGet     = "http://%s/pd/api/v1/leader"
	pdAPIV1MembersGet    = "http://%s/pd/api/v1/members"
)

func pdStoresGet(host string) (string, error) {
	bs, err := httputil.Get(fmt.Sprintf(pdAPIV1StoresGet, host), pdReqTimeout)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

func pdStoreDelete(host string, id int) error {
	if err := httputil.Delete(fmt.Sprintf(pdAPIV1StoreIDDelete, host, id), pdReqTimeout); err != nil {
		return err
	}
	logs.Warn(`store:%d deleted`, id)
	return nil
}

func pdLeaderGet(host string) (string, error) {
	bs, err := httputil.Get(fmt.Sprintf(pdAPIV1LeaderGet, host), pdReqTimeout)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

func pdMembersGet(host string) (string, error) {
	bs, err := httputil.Get(fmt.Sprintf(pdAPIV1MembersGet, host), pdReqTimeout)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

func pdMembersGetName(host string) ([]string, error) {
	s, err := pdMembersGet(host)
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

func pdMemberDelete(host string, name string) error {
	if err := httputil.Delete(fmt.Sprintf(pdAPIV1MembersGet+"/%s", host, name), pdReqTimeout); err != nil {
		return err
	}
	logs.Warn(`pd:%d deleted`, name)
	return nil
}
