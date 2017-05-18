#!/bin/bash

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/../../env.sh

echo "build pingcap/pd image"
echo "do tag-------------------------------------starting"
# remove --build-arg if no proxy on host
docker build --build-arg HTTPS_PROXY=$proxy -t $registry/ffan/rds/pd:$version ./