#!/bin/bash

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/../../dev.env

set -e

VERSION="latest"

echo "****************************"
echo "*Starting build tidb-gc image..."
echo "*  Proxy: $DPROXY"
echo "*  Image: $REGISTRY/ffan/rds/tidb-gc:$VERSION-base"
echo "****************************" 
# Build a fresh base ffan/rds/tidb-gc image
(docker build $DPROXY -t $REGISTRY/ffan/rds/tidb-gc:$VERSION-base -f dockerfile ../../)

# Extract files from ffan/rds/tidb-gc image
mkdir base
docker run -ti --rm -v $PWD/base:/base -u $UID $REGISTRY/ffan/rds/tidb-gc:$VERSION-base bash -c 'cp -R /go/bin/* /base/'

# Build ffan/rds/tidb-gc image
docker build -t $REGISTRY/ffan/rds/tidb-gc:$VERSION -f dockerfile_lite ./

# Clean up temporary files
rm -rf base