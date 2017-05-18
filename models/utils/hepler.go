package utils

import (
	"errors"
	"time"

	"github.com/astaxie/beego/logs"
)

var (
	errTimeout = errors.New("timeout")
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
		return errTimeout
	}
}

// Contains 返回slice是否包含指定的值
func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
