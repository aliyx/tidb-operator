#!/bin/bash

# set -x

if [ -f "/tmp/tidb-k8s.pid" ]
then
  pid=$(cat /tmp/tidb-k8s.pid)
  if [ ! -z "$pid" ]; then
    kill -9 $pid
    echo "stop old tidb-k8s server..."
  fi
  rm /tmp/tidb-k8s.pid
fi

function fail() {
  echo "ERROR: $1"
  exit 1
}

ip=$(/sbin/ifconfig eth0 | grep 'netmask ' | cut -d: -f2 | awk '{print $2}')
if [ -z "$ip" ]; then
  ip=0.0.0.0
fi
export HTTPAddr=$ip

# set env
e=$1
if [ -z "$e" ]; then
  echo -e "\033[33mCurrent environment: dev\033[0m"
  export RunMode=dev
  export ETCD_ADDRESS=http://10.213.44.128:12379
  export K8S_ADDRESS=http://10.213.44.128:10218
elif [ "$e" == "test" ]; then
  echo -e "\033[33mCurrent environment: test\033[0m"
  export RunMode=test
  export ETCD_ADDRESS=http://10.213.131.54:12379
  export K8S_ADDRESS=http://10.213.131.54:10218
elif [ "$e" == "prod" ]; then
  echo -e "\033[33mCurrent environment: prod\033[0m"
  export RunMode=prod
else
  echo -e "\033[31m No environment: $e\033[0m"
  exit 1
fi

top=$(pwd)
# top sanity check
if [[ "$top" == "${top/\/src\/github.com\/ffan\/tidb-k8s/}" ]]; then
  fail "top($top) does not contain src/github.com/ffan/tidb-k8s"
fi

go version 2>&1 >/dev/null || fail "Go is not installed or is not on \$PATH"

set -e

CGO_ENABLED=0 go build -ldflags '-d -w -s'

if [ !-f tidb-k8s ];then
  fail "build failed"
fi

# start
./tidb-k8s & echo $! > /tmp/tidb-k8s.pid