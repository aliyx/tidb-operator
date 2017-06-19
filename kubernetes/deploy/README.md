# Build kubernetes cluster

Install the kubernetes cluster in the company's local area network.

## Install

First upload k8s need 4 rpm package (*kubectl-1.6.1-0.x86_64.rpm, *kubeadm-1.6.1-0.x86_64.rpm, *kubelet-1.6.1-0.x86_64.rpm, *Kubernetes-cni-0.5.1-0.x86_64.rpm) to the server you want to install, and then execute the following script:

```bash
./install.sh . # .Indicates the location of rpm
```

## Master

Execute on the specified master node：

```bash
./kube-master.sh
```

## Node

Execute on the specified slve node：

```bash
./kube-node.sh ip # Ip refers to the master node ip address
```

## Set proxy and prometheus

```bash
# label proxy
kubectl label node name node-role.proxy=
kubectl taint nodes name node-role.proxy=:NoSchedule
# label prometheus
kubectl taint nodes name node-role.prometheus=:NoSchedule

#remove
kubectl label node name node-role.proxy-
kubectl taint nodes name node-role.proxy-
kubectl taint nodes name node-role.prometheus-
```

## Access

K8s apiserver exposed port is 10218, dashboard port is 12801, pod port range is 12800-14999, because the company's export port range is 10000-14999.
Visit dashboard: {masterip}:12801