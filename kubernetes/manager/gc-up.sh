#!/bin/bash

set -e

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/../env.sh

version=latest
DATA_VOLUME=${DATA_VOLUME:-''}
tidbdata_volume='emptyDir: {}'
if [ -n "$DATA_VOLUME" ]; then
  tidbdata_volume="hostPath: {path: ${DATA_VOLUME}}"
fi

 echo "Creating tidb gc daemonset..."
sed_script=""
for var in version tidbdata_volume; do
  sed_script+="s,{{$var}},${!var},g;"
done
cat gc-daemonset.yaml | sed -e "$sed_script" | $KUBECTL $KUBECTL_OPTIONS create -f -