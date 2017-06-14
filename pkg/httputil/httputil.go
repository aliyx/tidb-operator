package httputil

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"errors"
)

var (
	// ErrAlreadyExists 409返回该错误
	ErrAlreadyExists = errors.New("resource already exists")
	// ErrNotFound 404
	ErrNotFound = errors.New("resource not exists")
)

// Post 发送post请求
func Post(url string, body []byte) (string, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("error: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	r, _ := ioutil.ReadAll(resp.Body)
	sr := string(r)
	switch resp.StatusCode {
	case 200, 201:
		return sr, nil
	case 409:
		return "", ErrAlreadyExists
	default:
		return "", fmt.Errorf("create server %s error: %v", url, sr)
	}
}

// Get 发送get请求
func Get(url string, timeout time.Duration) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	client.Timeout = timeout
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	r, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode == 404 {
		return nil, ErrNotFound
	}
	// logs.Debug("http get statusCode: %d", resp.StatusCode)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("get server %s error: %v", url, string(r))
	}
	return r, nil
}

// Delete delete资源
func Delete(url string, timeout time.Duration) error {
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("error: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	client.Timeout = timeout
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	r, _ := ioutil.ReadAll(resp.Body)
	// 201:创建成功
	if resp.StatusCode != 200 && resp.StatusCode != 404 {
		return fmt.Errorf("delete service %s error: %v", url, string(r))
	}
	return nil
}

// Patch 修改资源
func Patch(url string, body []byte, timeout time.Duration) error {
	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("error: %v", err)
	}
	req.Header.Set("Content-Type", "application/merge-patch+json")

	client := &http.Client{}
	client.Timeout = timeout
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	r, _ := ioutil.ReadAll(resp.Body)
	sr := string(r)
	if resp.StatusCode != 200 && resp.StatusCode != 404 {
		return fmt.Errorf("http: %v error: %v", url, sr)
	}
	return nil
}

// GetOk 返回get请求是否成功
func GetOk(url string, timeout time.Duration) error {
	if _, err := Get(url, timeout); err != nil {
		return err
	}
	return nil
}
