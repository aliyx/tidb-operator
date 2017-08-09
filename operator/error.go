package operator

import "errors"
import (
	"strings"

	"k8s.io/client-go/pkg/api/v1"
)

var (
	errInvalidReplica = errors.New("invalid replica")

	// ErrRepeatOperation is returned by functions to specify the operation is executing.
	ErrRepeatOperation = errors.New("the previous operation is being executed, please stop first")
	// ErrScaling be scaling
	ErrScaling = errors.New("be scaling")
	// ErrUnavailable ...
	ErrUnavailable = errors.New("unavailable")
	// ErrUnsupportPatch ...
	ErrUnsupportPatch = errors.New("unsupport patch operator")
)

func parseError(db *Db, err error) {
	if err == nil || db == nil {
		return
	}
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, v1.PodReasonUnschedulable):
		db.Status.Reason = v1.PodReasonUnschedulable
		db.Status.Message = msg[len(v1.PodReasonUnschedulable)+1:]
	}
}
