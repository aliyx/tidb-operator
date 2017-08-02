#!/bin/bash

rm -rf base

set -e

VERSION=${VERSION:-'latest'}

echo "****************************"
echo "*Starting build tikv image..."
echo "*  Proxy: $DPROXY"
echo "*  Image: ${REGISTRY}tikv:$VERSION"
echo "****************************" 

echo "build mountpath"
mkdir bin
go build -o ./bin/mountpath ./mountpath

(docker build $DPROXY --build-arg VERSION=$VERSION -t ${REGISTRY}tikv:$VERSION-base -f dockerfile ./)

# Extract files from tikv image
mkdir base
docker run -ti --rm -v $PWD/base:/base -u $UID --entrypoint sh ${REGISTRY}tikv:$VERSION-base -c 'cp -f /tikv-server /base/tikv-server'

# Build tikv image
docker build -t ${REGISTRY}tikv:$VERSION -f dockerfile_lite ./

# Clean up temporary files
rm -rf base
rm -rf bin