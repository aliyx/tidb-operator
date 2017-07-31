#!/bin/bash

# This is an example script that stops tidb-operator.

export NS="default"

#set -e

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/../env.sh

echo "Stopping tidb-operator..."
$KUBECTL $KUBECTL_OPTIONS delete deployment,service,ClusterRoleBinding,ServiceAccount tidb-operator


