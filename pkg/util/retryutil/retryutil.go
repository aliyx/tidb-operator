package retryutil

import (
	"errors"
	"fmt"
	"time"

	"github.com/astaxie/beego/logs"
)

var (
	ErrRetryTimeout = errors.New("timeout")
)

type RetryError struct {
	n int
}

func (e *RetryError) Error() string {
	return fmt.Sprintf("still failing after %d retries", e.n)
}

func IsRetryFailure(err error) bool {
	_, ok := err.(*RetryError)
	return ok
}

type ConditionFunc func() (bool, error)

// Retry retries f every interval until after maxRetries.
// The interval won't be affected by how long f takes.
// For example, if interval is 3s, f takes 1s, another f will be called 2s later.
// However, if f takes longer than interval, it will be delayed.
func Retry(interval time.Duration, maxRetries int, f ConditionFunc) error {
	if maxRetries <= 0 {
		return fmt.Errorf("maxRetries (%d) should be > 0", maxRetries)
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()

	for i := 0; ; i++ {
		ok, err := f()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if i+1 == maxRetries {
			break
		}
		<-tick.C
	}
	return &RetryError{maxRetries}
}

type call func() error

// RetryIfErr 等待call调用成功，直到timeout
func RetryIfErr(timeout time.Duration, call call) error {
	c := make(chan interface{})
	go func() {
		defer close(c)
		for {
			select {
			case <-c:
				return
			default:
				if err := call(); err != nil {
					time.Sleep(time.Second)
					logs.Warn(`Waiting for success: %v`, err)
				} else {
					return
				}
			}
		}
	}()
	select {
	case <-c:
		return nil // completed normally
	case <-time.After(timeout):
		c <- 1
		return ErrRetryTimeout
	}
}
