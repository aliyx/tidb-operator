# Tidb-operator

tidb-operator manage multiple tidb cluster atop Kubernetes, support multi-tenant, and online scale up or dowm, rolling upgrades, multiple disk mount points, full / incremental migrate data to tidb cluster, all operations web.

Note: **Currently only support kubernetes version is `1.7`, all port ranges `[10000-15000)`. Only provide restful api, easy to integrate into the existing web frame.**

## Build images

Build all docker images and push to your private docker registry, default version is `latest`:

  ```bash
  make build && make push
  ```

## Preparedness

#### Install kubernetes if have already installed, skip this step

Note: Due to GFW reasons, some installation packages and images can not be obtained, you need to download to the local upload to the specified server and then install. See: kubernetes `./kubernetes/deploy` directory.

Access kubernetes dashboard: `http://<NodeIP>:10281`

#### Download

Git clone the project to `$GOPATH/src/github/ffan` dir

#### Deploy prometheus/grafana on kubernetes

```bash
make install-grafana # run this shell on kubernetes master
```

Access grafana: `http://<NodeIP>:12802`, user&password is admin/admin.

## Startup tidb-operator

### To start on kubernetes cluster

```bash
make install-tidb-operator # run this shell on kubernetes master
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