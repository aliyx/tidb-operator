#!/bin/bash

set -e

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/env.sh

namespace=$NS
cell=`echo $CELL`
version=${VERSION}
cpu=${PD_CPU:-200}
mem=${PD_MEM:-256}
replicas=${PD_REPLICAS:-3}
DATA_VOLUME=${DATA_VOLUME:-''}
tidbdata_volume='emptyDir: {}'
if [ -n "$DATA_VOLUME" ]; then
  tidbdata_volume="hostPath: {path: ${DATA_VOLUME}}"
fi

echo "Creating pd service for $cell cell..."
# cat pd-service.yaml | sed -e "$sed_script" | $KUBECTL $KUBECTL_OPTIONS create -f -

for id in `seq 1 $replicas`; do
  # Create the pod.
  sed_script=""
  for var in namespace cell id replicas cpu mem version tidbdata_volume registry; do
    sed_script+="s,{{$var}},${!var},g;"
  done
  echo "Creating pd pod $id for $cell cell..."
  cat pd-pod.yaml | sed -e "$sed_script"
  # cat pd-pod.yaml | sed -e "$sed_script" | $KUBECTL $KUBECTL_OPTIONS create -f -
done


