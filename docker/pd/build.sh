#!/bin/bash

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/../../env.sh

echo "build ffan/rds/pd image ..."
docker build $DPROXY -t $REGISTRY/ffan/rds/pd:$VERSION ./