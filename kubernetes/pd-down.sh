#!/bin/bash

# This is an example script that tears down the pd servers started by
# etcd-up.sh.

#set -e

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/env.sh

cell=`echo $CELL`

# Delete replicaSet
echo "Stopping pd replicationcontroller for $cell cell..."
$KUBECTL $KUBECTL_OPTIONS delete replicationcontroller pd-$cell

echo "Deleting pd service for $cell cell..."
$KUBECTL $KUBECTL_OPTIONS delete service pd-$cell
$KUBECTL $KUBECTL_OPTIONS delete service pd-$cell-srv

