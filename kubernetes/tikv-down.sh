#!/bin/bash

# This is an example script that tears down the tikv deployment started by
# tikv-up.sh.

#set -e

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/env.sh

replicas=${KV_REPLICAS:-3}

cell=`echo $CELL`

# Delete pod
for id in `seq 1 $replicas`; do
  echo "Deleting tikv pod $id for $cell cell..."
  $KUBECTL $KUBECTL_OPTIONS  delete pod tikv-$cell-$id
done

