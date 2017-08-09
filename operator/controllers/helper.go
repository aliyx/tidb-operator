package controllers

import (
	"github.com/ffan/tidb-operator/operator"
	"github.com/ffan/tidb-operator/pkg/storage"
)

func err2httpStatuscode(err error) (code int) {
	switch err {
	case storage.ErrNoNode:
		return 404
	case operator.ErrRepeatOperation:
		return 402
	case operator.ErrUnsupportPatch:
		return 403
	case storage.ErrAlreadyExists:
		return 409
	default:
		return 500
	}
}
