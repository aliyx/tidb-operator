package controllers

import (
	"github.com/ffan/tidb-operator/models"
	"github.com/ffan/tidb-operator/pkg/storage"
)

func err2httpStatuscode(err error) (code int) {
	switch err {
	case storage.ErrNoNode:
		return 404
	case models.ErrRepeatOperation:
		return 402
	default:
		return 500
	}
}
