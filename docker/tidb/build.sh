#!/bin/bash

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/env.sh

echo "build ffan/rds/tidb image ..."
docker build $DPROXY -t $REGISTRY/ffan/rds/tidb:$VERSION .