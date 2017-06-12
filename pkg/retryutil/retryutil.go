package retryutil

import (
	"errors"
	"time"

	"github.com/astaxie/beego/logs"
)

var (
	ErrRetryTimeout = errors.New("timeout")
)

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
