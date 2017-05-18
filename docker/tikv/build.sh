#!/bin/bash

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/../../env.sh

echo "build pingcap/tikv image"
# remove --build-arg if no proxy on host
docker build --build-arg HTTP_PROXY=$proxy --build-arg HTTPS_PROXY=$proxy -t $registry/ffan/rds/tikv:$version ./