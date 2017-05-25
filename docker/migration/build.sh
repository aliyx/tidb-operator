#!/bin/bash

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/../../dev.env

echo "****************************"
echo "*Starting build ffan/rds/migration image..."
echo "*  Proxy: $DPROXY"
echo "*  Image: $REGISTRY/ffan/rds/migration:$VERSION"
echo "****************************" 

docker build $DPROXY -t $REGISTRY/ffan/rds/migration:$VERSION ./