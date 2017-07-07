#!/bin/bash

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/../../dev.env

VERSION=${VERSION:-'latest'}

echo "****************************"
echo "*Starting build ffan/rds/migrator image..."
echo "*  Proxy: $DPROXY"
echo "*  Image: $REGISTRY/ffan/rds/migrator:$VERSION"
echo "****************************" 

docker build $DPROXY -t $REGISTRY/ffan/rds/migrator:$VERSION ./