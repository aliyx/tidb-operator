#!/bin/bash

script_root=$(dirname "${BASH_SOURCE}")
source $script_root/../../dev.env

rm -rf base

set -e

VERSION=${VERSION:-'latest'}

echo "****************************"
echo "*Starting build pd image..."
echo "*  Proxy: $DPROXY"
echo "*  Image: $REGISTRY/ffan/rds/pd:$VERSION"
echo "****************************"

branch=$VERSION
if [ "-skip-base" != "$1" ]; then
  if [ "$branch" == "latest" ]; then
    branch="master"
  fi
  (docker build $DPROXY --build-arg VERSION=$branch -t $REGISTRY/ffan/rds/pd:$VERSION-base -f dockerfile ./)
fi

# Extract files from ffan/rds/pd image
mkdir base
docker run -ti --rm -v $PWD/base:/base -u $UID $REGISTRY/ffan/rds/pd:$VERSION-base bash -c 'cp -R /go/bin/* /base/'

# Build ffan/rds/pd image
docker build -t $REGISTRY/ffan/rds/pd:$VERSION -f dockerfile_lite ./

# Clean up temporary files
rm -rf base