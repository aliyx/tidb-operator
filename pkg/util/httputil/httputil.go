package httputil

import (
	"fmt"
	"time"

	"errors"

	"gopkg.in/resty.v0"
)

var (
	// ErrAlreadyExists 409
	ErrAlreadyExists = errors.New("already exists")
	// ErrNotFound 404
	ErrNotFound = errors.New("not found")
)

// Post send post request
func Post(url string, body []byte) (string, error) {
	resp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		Post(url)

	if err != nil {
		return "", err
	}
	switch resp.StatusCode() {
	case 200, 201:
		return resp.String(), nil
	case 409:
		return "", ErrAlreadyExists
	default:
		return "", fmt.Errorf("http: %s error: %s", url, resp.String())
	}
}

// Get send get request
func Get(url string, timeout time.Duration) ([]byte, error) {
	resty.SetTimeout(timeout)
	resp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() == 404 {
		return nil, ErrNotFound
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("http: %s error: %s", url, resp.String())
	}
	return resp.Body(), nil
}

// Delete send delete request
func Delete(url string, timeout time.Duration) error {
	resty.SetTimeout(timeout)
	resp, err := resty.R().
		Delete(url)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 && resp.StatusCode() != 404 {
		return fmt.Errorf("http: %s error: %s", url, resp.String())
	}
	return nil
}

// Patch send patch request
func Patch(url string, body []byte, timeout time.Duration) error {
	resty.SetTimeout(timeout)
	resp, err := resty.R().
		SetHeader("Content-Type", "application/merge-patch+json").
		SetBody(body).
		Patch(url)
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 && resp.StatusCode() != 404 {
		return fmt.Errorf("http: %s error: %s", url, resp.String())
	}
	return nil
}

// GetOk resource is ok
func GetOk(url string, timeout time.Duration) error {
	if _, err := Get(url, timeout); err != nil {
		return err
	}
	return nil
}
