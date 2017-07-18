# tidb-operator

tidb-operator manage multiple tidb cluster atop Kubernetes, support for the actual needs of users to apply for different specifications of resources, supports online scale up or dowm, rolling upgrades, full / incremental migrate data to tidb cluster, all operations web.

## Build images

Build tidb docker images and push to private registry.

* Please configure your development environment `./dev.env`.

* Build docker private images, default version is latest:

```bash
./docker/tidb-gc/build.sh # build tidb-gc image
./docker/tidb-operator/build.sh # build tidb-operator image
./docker/prom-server/build.sh # build prom-server image for adding prometheus config to image
./docker/migrator/build.sh # build migrator image for supporting full / incremental migrate to tidb cluster

# build tidb
./docker/pd/build.sh
./docker/tikv/build.sh
./docker/tidb/build.sh
```

* Push pd/tikv/tidb... all images builded to your private registry.

## Preparedness

### Install kubernetes

Note: Due to GFW reasons, some installation packages and images can not be obtained, you need to download to the local upload to the specified server and then install. See: kubernetes `./kubernetes/deploy` directory.

Access kubernetes dashboard: {masterid}:10281

### Download

Git clone the project to `'$GOPATH/src/github/ffan` dir

### Deploy prometheus/grafana on kubernetes

```bash
./kubernetes/prometheus/deploy.sh # run this shell on kubernetes master
```

### Deploy tidb-gc on kubernetes

```bash
./kubernetes/manager/gc-up.sh # run this shell on kubernetes master
```

## Startup tidb-operator

### Local

```bash
cd ./operator && ln -s swagger ../swagger # ln swagger to `./tidb-operator`
```

```bash
bee run -downdoc=true # beego
# or
./restart.sh
```

Access endpoint: 127.0.0.1:12808/swagger

### Kubernetes

Please set your environment variable in `./kubernetes/env.sh` and run.

```bash
./kubernetes/tk-up.sh # deploy tidb-operator on kubernetes
```