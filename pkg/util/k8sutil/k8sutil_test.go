// Copyright 2016 The etcd-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8sutil

import (
	"fmt"
	"os"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func TestMain(m *testing.M) {
	masterHost = "http://10.213.44.128:10218"
	kubecli = MustNewKubeClient()
	os.Exit(m.Run())
}

func TestMustNewKubeClient(t *testing.T) {
	kc := MustNewKubeClient()
	pods, err := kc.CoreV1().Pods("").List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))
}

func TestGetNodesIP(t *testing.T) {
	sel := map[string]string{
		"node-role.proxy": "",
	}
	ips, err := GetNodesExternalIP(sel)
	if err != nil {
		t.Errorf("%v", err)
	}
	fmt.Printf("size:%d %s\n", len(ips), ips)
}

func TestGetEtcdIP(t *testing.T) {
	ip, err := GetEtcdIP()
	if err != nil {
		t.Errorf("%v\n", err)
	}
	println(ip)
}
