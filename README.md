# tidb-operator

tidb-operator manage multiple tidb cluster atop Kubernetes, support for the actual needs of users to apply for different specifications of resources, support online dynamic scale, all operations web.

## Build images

Build tidb docker images and push to private registry.

* Please configure your development environment `./dev.env`.

* Build docker private images:

```bash
./docker/tidb-gc/build.sh # build tidb-gc
./docker/prom-server/build.sh # add prometheus config to image
./docker/migration/build.sh # migrate mysql to tidb

# build tidb
./docker/pd/build.sh
./docker/tikv/build.sh
./docker/tidb/build.sh
```

* Push pd/tikv/tidb images to your private registry.

## Preparedness

### Install kubernetes

Note: Due to GFW reasons, some installation packages and images can not be obtained, you need to download to the local upload to the specified server and then install. See: kubernetes `./kubernetes/deploy` directory.

Access kubernetes dashboard: {masterid}:10281

### Deploy tidb-gc on kubernetes

```bash
./kubernetes/manager/gc-up.sh # run this shell on kubernetes master
```

## Startup tidb-operator

### Local

```bash
bee run -downdoc=true # beego
```

or

```bash
./restart.sh
```

Access endpoint: 127.0.0.1:12808/swagger

### Kubernetes

Please set your environment variable in `tk-up.sh`.

```bash
./kubernetes/tk-up.sh # run this shell on kubernetes master
```