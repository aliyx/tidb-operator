#!/bin/bash

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/../../dev.env

rm -rf base

set -e

VERSION=${VERSION:-'latest'}

echo "****************************"
echo "*Starting build tikv image..."
echo "*  Proxy: $DPROXY"
echo "*  Image: $REGISTRY/ffan/rds/tikv:$VERSION"
echo "****************************" 
(docker build $DPROXY -t $REGISTRY/ffan/rds/tikv:$VERSION-base -f dockerfile ./)

# Extract files from ffan/rds/pd image
mkdir base
docker run -ti --rm -v $PWD/base:/base -u $UID --entrypoint sh $REGISTRY/ffan/rds/tikv:$VERSION-base -c 'cp -f /tikv-server /base/tikv-server'

# Build ffan/rds/tikv image
docker build -t $REGISTRY/ffan/rds/tikv:$VERSION -f dockerfile_lite ./

# Clean up temporary files
rm -rf base