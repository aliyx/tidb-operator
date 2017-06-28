#!/bin/bash

set -e

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/../env.sh

echo "Deleting tidb gc daemonset..."
$KUBECTL $KUBECTL_OPTIONS delete daemonset tidb-gc