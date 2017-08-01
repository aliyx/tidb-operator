#!/bin/bash

VERSION=${VERSION:-'latest'}

echo "****************************"
echo "*Starting build migrator image..."
echo "*  Proxy: $DPROXY"
echo "*  Image: ${REGISTRY}migrator:$VERSION"
echo "****************************" 

docker build $DPROXY -t ${REGISTRY}migrator:$VERSION ./