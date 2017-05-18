#!/bin/bash

# This is an example script that deletes a namespace.

set -e

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/env.sh

namespace=${NS:-'tidb'}

echo "Deleting namespace $namespace..."
$KUBECTL $KUBECTL_OPTIONS delete namespace $namespace
