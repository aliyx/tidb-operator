package operator

import "errors"

var (
	errInvalidReplica = errors.New("invalid replica")

	// ErrRepeatOperation is returned by functions to specify the operation is executing.
	ErrRepeatOperation = errors.New("the previous operation is being executed, please stop first")
)
