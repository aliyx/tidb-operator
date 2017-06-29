#!/bin/bash

set -e

export NS="kube-system"

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/../env.sh

version=${VERSION}
registry=${REGISTRY}
env=${runMode:-'dev'}
initMd=${runMode:-'false'}

echo "****************************"
echo "*Creating tidb-operator namespace: $NS"
echo "*  Run mode: $env"
echo "****************************"

# Create the tidb-operator service and deployment.
sed_script=""
for var in initMd env version registry; do
  sed_script+="s,{{$var}},${!var},g;"
done
echo "Creating tidb-operator service/deployment..."
cat to-template.yaml | sed -e "$sed_script" | $KUBECTL $KUBECTL_OPTIONS create -f -

