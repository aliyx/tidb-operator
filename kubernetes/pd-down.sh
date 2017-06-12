#!/bin/bash

# This is an example script that tears down the pd servers started by
# etcd-up.sh.

#set -e

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/env.sh

cell=`echo $CELL`
replicas=${PD_REPLICAS:-3}

# Delete pod
for id in `seq 1 $replicas`; do
  id=$(printf "%03d\n" $id)
  echo "Deleting pd pod $id for $cell cell..."
  $KUBECTL $KUBECTL_OPTIONS  delete pod pd-$cell-$id
done

echo "Deleting pd service for $cell cell..."
$KUBECTL $KUBECTL_OPTIONS delete service pd-$cell
$KUBECTL $KUBECTL_OPTIONS delete service pd-$cell-srv

