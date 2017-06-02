package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("error:\n%v\n", r)
		}
		os.Stdout.Sync()
	}()
	cell := os.Args[1]
	if cell == "" {
		return
	}
	_, addrs, err := net.LookupSRV("pd-server", "tcp", fmt.Sprintf("pd-%s-srv", cell))
	if err != nil {
		fmt.Printf("lookup srv error:\n%v\n", err)
		return
	}

	localIP, err := getLocalIP()
	if err != nil {
		fmt.Printf("get ip error:\n%v\n", err)
	}
	cip := []string{}
	for _, addr := range addrs {
		id := strings.Split(addr.Target, ".")[0]
		if len(id) < 1 {
			fmt.Printf("error:\n%s\n", "target is nil")
			return
		}
		ips, err := net.LookupIP(addr.Target)
		if err != nil {
			fmt.Printf("error:\n%v\n", err)
			return
		}
		if len(ips) < 1 {
			fmt.Printf("error:\ncannt get ip for %s\n", addr.Target)
			return
		}
		ip := ips[0].String()
		cip = append(cip, ip)
		if localIP == ip {
			fmt.Printf("%s\n", id)
			return
		}
	}
	fmt.Printf("local:%s\nsrv:%v\n", localIP, cip)
}

func getLocalIP() (string, error) {
	host, err := os.Hostname()
	if err != nil {
		return "", err
	}
	addrs, err := net.LookupIP(host)
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ipv4 := addr.To4(); ipv4 != nil {
			return ipv4.String(), nil
		}
	}
	return "", errors.New("cannt get host ip")
}
