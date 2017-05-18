#!/bin/bash

set -e

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/env.sh

version=${VERSION}
registry=${REGISTRY}
etcd=${ETCD_GLOBAL}
k8s=${K8sAddr:-'http://10.213.44.128:10218'}
# Create the tidb-k8s service and deployment.
sed_script=""
for var in etcd k8s version registry; do
  sed_script+="s,{{$var}},${!var},g;"
done
echo "Creating tidb-k8s service/deployment..."
cat tk-template.yaml | sed -e "$sed_script" | $KUBECTL $KUBECTL_OPTIONS create -f -

