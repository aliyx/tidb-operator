# tidb-operator

tidb-operator manage multiple tidb cluster atop Kubernetes, support for the actual needs of users to apply for different specifications of resources, support online dynamic scale, all operations web.

## Build images

Build tidb docker images and push to private registry.

* Please configure your development environment `./dev.env`.

* Build docker iamges:

```bash
./docker/pd/build.sh
./docker/tikv/build.sh
./docker/tidb/build.sh
```

* Push pd/tikv/tidb images to your private registry.

## Preparedness

### Install kubernetes

Note: Due to GFW reasons, some installation packages and images can not be obtained, you need to download to the local upload to the specified server and then install. See: kubernetes `./kubernetes/deploy` directory.

Access kubernetes dashboard: {masterid}:10281

## Startup

### Startup tidb-operator on local

```bash
bee run -downdoc=true # beego
./restart.sh
```

or

```bash
./restart.sh
```

### Startup tidb-operator on kubernetes

Please set your environment variable in `tk-up.sh`.

```bash
./kubernetes/tk-up.sh # run this shell on kubernetes master
```

Access endpoint: 127.0.0.1:12808/swagger