#!/bin/bash

# This is an example script that starts a tidb replicationcontroller.

set -e

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/env.sh

version=${VERSION}
cpu=${DB_CPU:-200}
mem=${DB_MEM:-256}
replicas=${DB_REPLICAS:-2}
registry=${REGISTRY}
namespace=$NS

cell=`echo $CELL`
# Create the replication controller.
sed_script=""
for var in cell replicas cpu mem version namespace registry; do
  sed_script+="s,{{$var}},${!var},g;"
done
echo "Creating tidb service/replicationcontroller for $cell cell..."
cat tidb-template.yaml | sed -e "$sed_script" | $KUBECTL $KUBECTL_OPTIONS create -f -
