#!/bin/bash

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/env.sh

echo "build pingcap/tidb image"
# remove --build-arg if no proxy on host
docker build --build-arg http_proxy=$proxy --build-arg https_proxy=$proxy -t $registry/ffan/rds/tidb:$version .