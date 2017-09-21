#!/bin/bash

set -e

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/../env.sh

namespace=$NS
version=${VERSION}
registry=${REGISTRY}
env=${RUN_MODE:-'dev'}
initMd=${INIT_MD:-'false'}
hostPath=${DATA_VOLUME}
mount=${MOUNT}
# default log level is info:6
logLevel=${LOG_LEVEL:-6}

echo "****************************"
echo "*Creating tidb-operator namespace: $NS"
echo "*Run mode: $env"
echo "****************************"

# Create the tidb-operator service and deployment.
sed_script=""
for var in initMd env version registry namespace hostPath mount logLevel; do
  sed_script+="s,{{$var}},${!var},g;"
done
echo "Creating tidb-operator service/deployment..."
cat op-template.yaml | sed -e "$sed_script"
# cat op-template.yaml | sed -e "$sed_script" | $KUBECTL $KUBECTL_OPTIONS create -f -

