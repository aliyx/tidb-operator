#!/bin/bash

rm -rf base

set -e

VERSION=${VERSION:-'latest'}

echo "****************************"
echo "*Starting build tidb image..."
echo "*  Proxy: $DPROXY"
echo "*  Image: ${REGISTRY}/tidb:$VERSION"
echo "****************************" 

branch=$VERSION
if [ "-skip-base" != "$1" ]; then
  if [ "$branch" == "latest" ]; then
    branch="master"
  fi
  (docker build $DPROXY --build-arg VERSION=$branch -t ${REGISTRY}/tidb:$VERSION-base -f dockerfile ./)
fi

# Extract files from tidb image
mkdir base
docker run -ti --rm -v $PWD/base:/base -u $UID ${REGISTRY}/tidb:$VERSION-base bash -c 'cp -f /tidb-server /base/tidb-server'

# Build tidb image
docker build -t ${REGISTRY}/tidb:$VERSION -f dockerfile_lite ./

# Clean up temporary files
rm -rf base