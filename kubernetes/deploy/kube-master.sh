#!/bin/bash

set -e

# master
sudo kubeadm reset
sudo kubeadm init --config master.yaml

# To start using your cluster, you need to run (as a regular user):
sudo cp /etc/kubernetes/admin.conf $HOME/
sudo chown $(id -u):$(id -g) $HOME/admin.conf
export KUBECONFIG=$HOME/admin.conf

until kubectl cluster-info
do
  sleep 1
done

# set api-server
echo 'export KUBERNETES_API_SERVER=127.0.0.1:10218' | sudo tee /etc/profile.d/tidb-operator.sh
echo 'alias kubectl=kubectl --server=127.0.0.1:10218' | sudo tee -a /etc/profile.d/tidb-operator.sh

# see logs
# journalctl -xeu kubelet

# set pod network
# use weave as pod network https://www.weave.works/docs/net/latest/kube-addon/
# kubectl apply -f https://github.com/weaveworks/weave/releases/download/latest_release/weave-daemonset-k8s-1.6.yaml
kubectl apply -f weave-daemonset-k8s-1.6.yaml

kubectl apply -f kubernetes-dashboard.yaml

# kubectl get pods,rc,DaemonSet -n kube-system
# kubectl get service kubernetes-dashboard -n kube-system -o=yaml