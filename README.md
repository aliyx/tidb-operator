# tidb-operator

tidb-operator manage multiple tidb cluster atop Kubernetes, support for the actual needs of users to apply for different specifications of resources, supports online scale up or dowm, rolling upgrades, full / incremental migrate data to tidb cluster, all operations web.

Note: Currently only support kubernetes version is `1.6`, all port ranges `[10000-15000)`.  **Only provide restful api, easy to integrate into the existing web frame.**

## Build images

Build all the images needed by the project and push to yourself private docker registry.

- Please modify docker private register or set http proxy if need in `./dev.env`.

- Build all docker images, default version is `latest`:

    ```bash
    # Build tidb-gc image, for recyling tikvs deleted and delete prometheus metrics, etc.
    ./docker/tidb-gc/build.sh
    # Build tidb-operator image, for create/delete/scale/upgrade tidb cluster.
    ./docker/tidb-operator/build.sh
    # Build prom-server image, for adding prometheus config to image
    ./docker/prom-server/build.sh
    # Build migrator image, for supporting full/incremental migrate mysql data to tidb cluster
    ./docker/migrator/build.sh

    # Build pd/tikv/tidb images, such as add some config.toml to image.
    # The official image on docker.com doesn't have
    ./docker/pd/build.sh
    ./docker/tikv/build.sh
    ./docker/tidb/build.sh
    ```

- Push pd/tikv/tidb... all images builded to your private docker registry.

## Preparedness

### Install kubernetes if have already installed, skip this step

Note: Due to GFW reasons, some installation packages and images can not be obtained, you need to download to the local upload to the specified server and then install. See: kubernetes `./kubernetes/deploy` directory.

Access kubernetes dashboard: `http://{masterIP}:10281`

### Download

Git clone the project to `$GOPATH/src/github/ffan` dir

### Deploy prometheus/grafana on kubernetes

```bash
./kubernetes/prometheus/deploy.sh # run this shell on kubernetes master
```

Access grafana: `http://{masterIP}:12802`, user/password is admin/admin.

### Deploy tidb-gc on kubernetes

```bash
./kubernetes/manager/gc-up.sh # run this shell on kubernetes master
```

## Startup tidb-operator

### Kubernetes

Please set your environment variable in `./kubernetes/env.sh` and run.

```bash
./kubernetes/manager/op.sh # deploy tidb-operator on kubernetes
```

### Development

```bash
cd ./operator && ln -s swagger ../swagger # ln swagger to `./tidb-operator`
```

```bash
# beego, set the kubenetes API server endpoint before running,
# for example `export K8S_ADDRESS=http://10.213.131.54:10218` or midify restart.sh
bee run -downdoc=true
# or
./restart.sh
```

Access endpoint: `http://127.0.0.1:12808/swagger`