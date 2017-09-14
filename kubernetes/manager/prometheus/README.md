# prometheus-kubernetes

![reference](https://github.com/migmartri/prometheus-kubernetes)

Prometheus setup that contains:

- One instance of prometheus server configured to scrap metrics from a prometheus push gateway service.
- Prometheus push gateway service used for receiving short lived jobs.
- Grafana dashboard

## Deploy

* Build prom-server image and push it into a container registry

```bash
./docker/prom-server/build.sh
```

* Deploy setup into k8s.

```bash
make install grafana # run this shell on kubernetes master
```

Access grafana: `http://<NodeIP>:12802`, user&password is admin/admin.