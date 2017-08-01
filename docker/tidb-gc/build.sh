#!/bin/bash

rm -rf base

set -e

VERSION=${VERSION:-'latest'}

echo "****************************"
echo "*Starting build tidb-gc image..."
echo "*  Proxy: $DPROXY"
echo "*  Image: ${REGISTRY}tidb-gc:$VERSION-base"
echo "****************************" 
# Build a fresh base tidb-gc image
(docker build $DPROXY -t ${REGISTRY}tidb-gc:$VERSION-base -f dockerfile ../../)

# Extract files from tidb-gc image
mkdir base
docker run -ti --rm -v $PWD/base:/base -u $UID ${REGISTRY}tidb-gc:$VERSION-base bash -c 'cp -R /go/bin/* /base/'

# Build tidb-gc image
docker build -t ${REGISTRY}tidb-gc:$VERSION -f dockerfile_lite ./

# Clean up temporary files
rm -rf base