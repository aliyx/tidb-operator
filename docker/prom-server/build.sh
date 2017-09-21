#!/bin/bash

VERSION=${VERSION:-'latest'}

echo "****************************"
echo "*Starting build prom-server image..."
echo "*  Proxy: $DPROXY"
echo "*  Image: ${REGISTRY}/migrator:$VERSION"
echo "****************************" 

docker build $DPROXY -t ${REGISTRY}/prom-server:$VERSION ./