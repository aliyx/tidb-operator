package main

import (
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	cell := os.Args[1]
	_, addrs, err := net.LookupSRV("pd-server", "tcp", fmt.Sprintf("%s", cell))
	if err != nil {
		return
	}

	for _, addr := range addrs {
		id := strings.Split(addr.Target, ".")[0]
		ips, err := net.LookupIP(addr.Target)
		if err != nil {
			return
		}
		ip := ips[0].String()
		fmt.Printf("%s=http://%s:%d\n", id, ip, addr.Port)
	}
}
