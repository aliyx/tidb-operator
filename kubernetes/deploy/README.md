# Build kubernetes cluster

Install the kubernetes cluster in the company's local area network.

## Start repository

Store various installation scripts and rpm for kube cluster install.

```bash
./repository.sh start
```

## Master

Execute on the specified master node：

```bash
bash <(curl -sSL 10.213.44.128:12701/kube/k8s-1.7.4.sh) master
```

## Node

Execute on the specified slve node：

```bash
bash <(curl -sSL 10.213.44.128:12701/kube/k8s-1.7.4.sh) join ip # Ip refers to the master node ip address
```

## Set proxy and prometheus

```bash
# label proxy
kubectl label node name node-role.proxy=
kubectl taint node name node-role.proxy=:PreferNoSchedule
# label prometheus
kubectl label node name node-role.prometheus=
kubectl taint node name node-role.prometheus=:PreferNoSchedule

#remove
kubectl label node name node-role.proxy-
kubectl taint node name node-role.proxy-
kubectl label node name node-role.prometheus-
kubectl taint node name node-role.prometheus-
```

## Access

K8s apiserver exposed port is 10218, dashboard port is 12801, pod port range is 12800-14999, because the company's export port range is 10000-14999.
Visit dashboard: {masterip}:12801