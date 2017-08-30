#!/bin/bash

#set -e

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/env.sh

version=${VERSION}
cpu=${KV_CPU:-200}
mem=${KV_MEM:-256}
capacity=${CAPACITY:-10}
replicas=${KV_REPLICAS:-3}
registry=${REGISTRY}
DATA_VOLUME=${DATA_VOLUME:-''}
namespace=$NS

((capacity=$capacity*1024*1024*1024))
cell=`echo $CELL`
tidbdata_volume='emptyDir: {}'
if [ -n "$DATA_VOLUME" ]; then
  tidbdata_volume="hostPath: {path: ${DATA_VOLUME}}"
fi
mount='data'

for id in `seq 1 $replicas`; do
  echo "Creating tikv pod $id for $cell cell..."
  id=$(printf "%03d" $id)
  sed_script=""
  for var in cell id cpu mem capacity version tidbdata_volume mount namespace registry; do
    sed_script+="s,{{$var}},${!var},g;"
  done
  # echo "Creating tikv deployment $id for $cell cell..."
  # cat tikv-deployment.yaml | sed -e "$sed_script" | $KUBECTL $KUBECTL_OPTIONS create -f -
  cat tikv-pod.yaml | sed -e "$sed_script" | $KUBECTL $KUBECTL_OPTIONS create -f -
done

