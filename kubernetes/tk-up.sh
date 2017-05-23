#!/bin/bash

set -e

export NS="kube-system"

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/env.sh

# local ip
ip="http://$(/sbin/ifconfig eth0 | grep 'netmask ' | cut -d: -f2 | awk '{print $2}'):10218"

etcd=${ETCD-'http://10.213.44.128:12379'}
version=${VERSION}
registry=${REGISTRY}
k8s=${K8sAddr:-$ip}
env=${RunMode:-'dev'}

echo "****************************"
echo "*Creating tidb-k8s namespace: $NS"
echo "*  Etcd address: $etcd"
echo "*  Run mode: $env"
echo "****************************"

# Create the tidb-k8s service and deployment.
sed_script=""
for var in etcd k8s version registry env; do
  sed_script+="s,{{$var}},${!var},g;"
done
echo "Creating tidb-k8s service/deployment..."
cat tk-template.yaml | sed -e "$sed_script" | $KUBECTL $KUBECTL_OPTIONS create -f -

