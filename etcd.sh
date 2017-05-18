#!/bin/bash

docker run -d --net=host \
    --name etcd-v3.1.5 \
    --volume=/var/lib/etcd-global:/var/lib/etcd-global \
    10.209.224.13:10500/quay.io/coreos/etcd:v3.1.5 \
    /usr/local/bin/etcd \
    --name etcd-global-1 \
    --data-dir /var/lib/etcd-global \
    --listen-client-urls http://0.0.0.0:12379 \
    --advertise-client-urls http://0.0.0.0:12379 \
    --listen-peer-urls http://0.0.0.0:12380 \
    --initial-advertise-peer-urls http://0.0.0.0:12380 \
    --initial-cluster etcd-global-1=http://0.0.0.0:12380 \
    --initial-cluster-token my-etcd-token \
    --auto-compaction-retention 1

# docker run -it ${image} /bin/sh
docker exec $(docker ps -a|grep etcd-v3.1.5|awk '{print $1}') /bin/sh -c "export ETCDCTL_API=3 && /usr/local/bin/etcdctl --endpoints=10.213.44.128:12379 --help"