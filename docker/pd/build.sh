#!/bin/bash

script_root=$(dirname "${BASH_SOURCE}")
source $script_root/../../dev.env

echo "****************************"
echo "*Starting build ffan/rds/pd image..."
echo "*  Proxy: $DPROXY"
echo "*  Image: $REGISTRY/ffan/rds/pd:$VERSION"
echo "****************************"
docker build $DPROXY -t $REGISTRY/ffan/rds/pd:$VERSION ./ && docker push $REGISTRY/ffan/rds/pd:$VERSION