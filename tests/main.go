package main

import (
	"context"
	"log"
	"time"

	"fmt"

	"github.com/coreos/etcd/clientv3"
)

func main() {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := cli.Put(ctx, "/sample_key/1", "sample_value"); err != nil {
		log.Println(err)
	}

	resp, err := cli.Get(ctx, "/sample_key")
	if err != nil {
		log.Println(err)
	}

	fmt.Printf("%v", resp)

	// cfg := client.Config{
	// 	Endpoints: []string{"http://127.0.0.1:2379"},
	// 	Transport: client.DefaultTransport,
	// 	// set timeout per request to fail fast when the target endpoint is unavailable
	// 	HeaderTimeoutPerRequest: time.Second,
	// }
	// c, err := client.New(cfg)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// kapi := client.NewKeysAPI(c)
	// // set "/foo" key with "bar" value
	// log.Print("Setting '/foo' key with 'bar' value")
	// resp, err := kapi.Set(context.Background(), "/foo", "bar", nil)
	// if err != nil {
	// 	log.Fatal(err)
	// } else {
	// 	// print common key info
	// 	log.Printf("Set is done. Metadata is %q\n", resp)
	// }
	// // get "/foo" key's value
	// log.Print("Getting '/foo' key value")
	// resp, err = kapi.Get(context.Background(), "/foo", nil)
	// if err != nil {
	// 	log.Fatal(err)
	// } else {
	// 	// print common key info
	// 	log.Printf("Get is done. Metadata is %q\n", resp)
	// 	// print value
	// 	log.Printf("%q key has %q value\n", resp.Node.Key, resp.Node.Value)
	// }
}
