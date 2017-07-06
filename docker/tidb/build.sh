#!/bin/bash

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/../../dev.env

rm -rf base

set -e

VERSION=${VERSION:-'latest'}

echo "****************************"
echo "*Starting build tidb image..."
echo "*  Proxy: $DPROXY"
echo "*  Image: $REGISTRY/ffan/rds/tidb:$VERSION"
echo "****************************" 
(docker build $DPROXY -t $REGISTRY/ffan/rds/tidb:$VERSION-base -f dockerfile ./)

# Extract files from ffan/rds/tidb image
mkdir base
docker run -ti --rm -v $PWD/base:/base -u $UID $REGISTRY/ffan/rds/tidb:$VERSION-base bash -c 'cp -f /tidb-server /base/tidb-server'

# Build ffan/rds/tidb image
docker build -t $REGISTRY/ffan/rds/tidb:$VERSION -f dockerfile_lite ./

# Clean up temporary files
rm -rf base