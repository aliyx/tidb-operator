#!/bin/bash
if [ -e /tmp/tidb-k8s.pid ]
then
  pid=$(cat /tmp/tidb-k8s.pid)
  if [ ! -z "$pid" ]; then
    kill -9 $pid
    echo "stop old tidb-k8s server..."
  fi
fi

function fail() {
  echo "ERROR: $1"
  exit 1
}

top=$(pwd)
# top sanity check
if [[ "$top" == "${top/\/src\/github.com\/ffan\/tidb-k8s/}" ]]; then
  fail "top($top) does not contain src/github.com/ffan/tidb-k8s"
fi

go version 2>&1 >/dev/null || fail "Go is not installed or is not on \$PATH"

CGO_ENABLED=0 go build -ldflags '-d -w -s'
rm /tmp/tidb-k8s.pid

export RunMode=dev
export EtcdAddress=http://10.213.44.128:12379
export K8sAddr=http://10.213.44.128:10218

./tidb-k8s & echo $! > /tmp/tidb-k8s.pid