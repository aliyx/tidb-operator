#!/bin/bash

# This is an example script that stops tidb.

#set -e

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/env.sh

cell=`echo $CELL`

# Delete rc & srv
echo "Stopping tidb replicationcontroller in cell $cell..."
$KUBECTL $KUBECTL_OPTIONS delete replicationcontroller tidb-$cell

echo "Deleting tidb service in cell $cell..."
$KUBECTL $KUBECTL_OPTIONS delete service tidb-$cell

