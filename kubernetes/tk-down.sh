#!/bin/bash

# This is an example script that stops tidb.

#set -e

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/env.sh

# Delete srv & deployment
echo "Stopping tidb-k8s deployment..."
$KUBECTL $KUBECTL_OPTIONS delete deployment tidb-k8s

echo "Deleting tidb-k8s service..."
$KUBECTL $KUBECTL_OPTIONS delete service tidb-k8s

