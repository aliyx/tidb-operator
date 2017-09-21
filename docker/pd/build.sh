#!/bin/bash

rm -rf base

set -e

VERSION=${VERSION:-'latest'}

echo "****************************"
echo "*Starting build pd image..."
echo "*  Proxy: $DPROXY"
echo "*  Image: ${REGISTRY}/pd:$VERSION"
echo "****************************"

branch=$VERSION
if [ "-skip-base" != "$1" ]; then
  if [ "$branch" == "latest" ]; then
    branch="master"
  fi
  (docker build $DPROXY --build-arg VERSION=$branch -t ${REGISTRY}/pd:$VERSION-base -f dockerfile ./)
fi

# Extract files from pd image
mkdir base
docker run -ti --rm -v $PWD/base:/base -u $UID ${REGISTRY}/pd:$VERSION-base bash -c 'cp -R /go/bin/* /base/'

# Build pd image
docker build -t ${REGISTRY}/pd:$VERSION -f dockerfile_lite ./

# Clean up temporary files
rm -rf base