#!/bin/bash

# This is an example script that stops tidb-operator.

export NS="kube-system"

#set -e

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/env.sh

# Delete srv & deployment
echo "Stopping tidb-operator deployment..."
$KUBECTL $KUBECTL_OPTIONS delete deployment tidb-operator

echo "Deleting tidb-operator service..."
$KUBECTL $KUBECTL_OPTIONS delete service tidb-operator

echo "Deleting tidb-operator clusterRoleBinding..."
$KUBECTL $KUBECTL_OPTIONS delete ClusterRoleBinding tidb-operator

echo "Deleting tidb-operator clusterRoleBinding..."
$KUBECTL $KUBECTL_OPTIONS delete ServiceAccount tidb-operator

