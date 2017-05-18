package controllers

import (
	"github.com/ffan/tidb-k8s/models"
)

func err2httpStatuscode(err error) (code int) {
	switch err {
	case models.ErrNoNode:
		return 404
	case models.ErrRepop:
		return 402
	default:
		return 500
	}
}
