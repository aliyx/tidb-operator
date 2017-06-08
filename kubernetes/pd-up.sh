#!/bin/bash

set -e

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/env.sh

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

# Create the client service and replication controller.
sed_script=""
for var in cell replicas cpu mem version tidbdata_volume; do
  sed_script+="s,{{$var}},${!var},g;"
done

echo "Creating pd service/replicationcontroller for $cell cell..."
cat pd-template.yaml | sed -e "$sed_script" | $KUBECTL $KUBECTL_OPTIONS create -f -

