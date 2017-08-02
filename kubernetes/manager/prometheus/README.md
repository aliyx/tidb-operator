# prometheus-kubernetes

https://github.com/migmartri/prometheus-kubernetes

Prometheus setup that contains:

- One instance of prometheus server configured to scrap metrics from a prometheus push gateway service.
- Prometheus push gateway service used for receiving short lived jobs.
- Grafana dashboard

## Deploy

* Build prom-server image and push it into a container registry

```bash
cd prom-server
./build.sh
```

* Deploy setup into k8s.

```bash
# From the project root execute
./up.sh
```

* Configure Grafana

You can find the IP address executing:

```bash
kubectl describe services grafana | grep NodePort
```

Once inside, create a data source using the endpoint:

```bash
URL: http://prom-server:9090/
Access: Proxy
```

### Caveats

Since we are using gcepersistentdisk, the number of pods that can mount
that volume type is limited to 1. That means that the number of replicas
for prom-server replication controller needs to be fixed to 1.

This also affects the ability of executing rolling-updates on that rc.

TODO: Explore other persistence layer options.
