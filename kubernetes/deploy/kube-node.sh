#!/bin/bash

set -e

master=$1
if [ -z "$master" ]; then
  echo "Please specify master ip: ./kube-node.sh ip"
  exit 1
fi
# set --skip-preflight-checks, because not find host by dns
sudo kubeadm reset
until sudo kubeadm join --token 997ea0.40e5c1218d0afc50 $master:6443
do
  sleep 1
done