# Tidb-k8s

Tidb-k8s manage multiple tidb cluster atop Kubernetes, support for the actual needs of users to apply for different specifications of resources, support online dynamic scale, all operations web.

## Preparedness

### Install kubernetes

Note: Due to GFW reasons, some installation packages and images can not be obtained, you need to download to the local upload to the specified server and then install. See: kubernetes / deploy directory.

### Install etcd

As tidb-k8s project back-end storage using the etcd cluster database, so the deployment of k8s-tidb need to install etcd.

```bash
docker run -d --net=host \
    --name etcd-v3.1.5 \
    --volume=/var/lib/tidb/etcd-1:/etcd-data \
    {docker_registry}/etcd:v3.1.5 \
    /usr/local/bin/etcd \
    --name etcd-1 \
    --data-dir /etcd-data \
    --listen-client-urls http://0.0.0.0:12379 \
    --advertise-client-urls http://0.0.0.0:12379 \
    --listen-peer-urls http://0.0.0.0:12380 \
    --initial-advertise-peer-urls http://0.0.0.0:12380 \
    --initial-cluster my-etcd-1=http://0.0.0.0:12380 \
    --initial-cluster-token my-etcd-token \
    --auto-compaction-retention 1
```

## Build images

Build tidb docker image and push to private registry.

```bash
./docker/pd/build.sh
./docker/tikv/build.sh
./docker/tidb/build.sh
```

## Startup

```bash
./restart.sh
```

Access local endpoint: 127.0.0.1:10228/swagger

## Topology

tidb-k8s project back-end storage using the etcd cluster database,tidb root: /dbs/tidb

- User path: $root/users/{id}/{cell}, Id is the associated user name, cell is the name of the created tidb.

- Metadata path: $root/metadata, Metadata information, the first start will initialize some of the default data (see: metadata.go), currently only supports Put operation, does not support Post / Delet and other operations.

- Tidb path: $root/tidbs/{cell}, The path under the storage tidb specific instance of information.

- Event path: $root/events/{cell}, Record tidb create / scale process.


