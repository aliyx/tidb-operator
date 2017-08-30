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
registry=${REGISTRY}
tidbdata_volume='emptyDir: {}'
c_state="new"
c_urls="--initial-cluster=\$urls"

pods=$($KUBECTL $KUBECTL_OPTIONS get pods -l cell=test|grep pd-test|wc -l)
if [ "$pods" -gt "0" ]; then
  cluster="--join=http://pd-$cell:2379"
else
  cluster="--initial-cluster=\$urls"
fi

echo "Creating pd pod for $cell cell..."
for id in `seq 1 $replicas`; do
  id=$(printf "%03d\n" $id)
  sed_script=""
  for var in namespace cell id replicas cpu mem version tidbdata_volume registry cluster; do
    sed_script+="s,{{$var}},${!var},g;"
  done
  sed_script+="s,{{c-state}},${c_state},g;"
  sed_script+="s,{{c-urls}},${c_urls},g;"
  # cat pd-pod.yaml | sed -e "$sed_script"
  cat pd-pod.yaml | sed -e "$sed_script" | $KUBECTL $KUBECTL_OPTIONS create -f -
done

echo "Creating pd service for $cell cell..."
sed_script="s,{{cell}},${cell},g;"
# cat pd-service.yaml | sed -e "$sed_script"
cat pd-service.yaml | sed -e "$sed_script" | $KUBECTL $KUBECTL_OPTIONS create -f -