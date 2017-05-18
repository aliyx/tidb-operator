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

# see logs
# journalctl -xeu kubelet

# set pod network
# use weave as pod network https://www.weave.works/docs/net/latest/kube-addon/
# kubectl apply -f https://github.com/weaveworks/weave/releases/download/latest_release/weave-daemonset-k8s-1.6.yaml
kubectl apply -f weave-daemonset-k8s-1.6.yaml

kubectl apply -f kubernetes-dashboard.yaml

# kubectl get pods,rc,DaemonSet -n kube-system
# kubectl get service kubernetes-dashboard -n kube-system -o=yaml