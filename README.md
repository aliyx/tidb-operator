# tidb-k8s

Tidb-k8s manager multiple tidb cluster atop Kubernetes, support for the actual needs of users to apply for different specifications of resources, support online dynamic expansion, all operations web.

## 前期准备工作

### Install kubernetes

Note: 由于GFW的原因，有些安装包和images无法获取，需要先下载到本地上传到指定的服务器再安装.  详见：kubernetes/deploy目录.

### Install etcd

由于tidb-k8s项目后端存储采用的是etcd cluster database,所以在部署k8s-tidb之前需要安装etcd

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

## 构建images

构建tidb docker image,并push到私有仓库

```bash
./docker/pd/build.sh
./docker/tikv/build.sh
./docker/tidb/build.sh
```

## 启动

```bash
./restart.sh
```

访问：127.0.0.1:10228/swagger

## Topology

tidb-k8s后端存储采用etcd服务，tidb的root为：/dbs/tidb

- user的path: $root/users/{id}/{cell}, id是关联的用户名称，cell是创建的tidb名称
- metadata的path: $root/metadata, 元数据信息，第一次启动时会初始化一些默认的数据(见：metadata.go)，目前只支持Put操作,不支持Post/Delet等操作
- tidb的path: $root/tidbs/{cell}, 该path下存放tidb具体的实例信息
- event的path: $root/events/{cell}, 记录tidb创建/scale过程


