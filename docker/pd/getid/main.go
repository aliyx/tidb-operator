package main

import (
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	cell := os.Args[1]
	_, addrs, err := net.LookupSRV("pd-server", "tcp", fmt.Sprintf("pd-%s-srv", cell))
	if err != nil {
		fmt.Printf("error: \n%v", err)
		return
	}

	localIP := getLocalIP()
	cip := []string{}
	for _, addr := range addrs {
		id := strings.Split(addr.Target, ".")[0]
		if len(id) < 1 {
			fmt.Printf("error: \n%s\n", "target is nil")
		}
		ips, err := net.LookupIP(addr.Target)
		if err != nil {
			fmt.Printf("error: \n%v\n", err)
			return
		}
		if len(ips) < 1 {
			fmt.Printf("error:\n cannt get ip for %s\n", addr.Target)
			return
		}
		ip := ips[0].String()
		cip = append(cip, ip)
		if localIP == ip {
			fmt.Printf("%s\n", id)
			return
		}
	}
	fmt.Printf("local:%s\n srv:%v\n", localIP, cip)
}

func getLocalIP() string {
	host, _ := os.Hostname()
	addrs, _ := net.LookupIP(host)
	for _, addr := range addrs {
		if ipv4 := addr.To4(); ipv4 != nil {
			return ipv4.String()
		}
	}
	return ""
}
