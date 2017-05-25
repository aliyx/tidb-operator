#!/bin/bash

set -e

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/env.sh

version=${VERSION}
sh=${SRC_HOST}
sP=${SRC_PORT}
su=${SRC_USER}
sp=${SRC_PASSWORD}
db=${SRC_DB}

dh=${DEST_HOST}
dP=${DEST_PORT}
duser=${DEST_USER}
dp=${DEST_PASSWORD}

registry=${REGISTRY}
cell=`echo $CELL`
# Create the client service and replication controller.
sed_script=""
for var in cell version registry sh sP su sp db dh dP duser dp; do
  sed_script+="s,{{$var}},${!var},g;"
done
echo "Creating migration pod for $cell cell..."
cat migration-pod.yaml | sed -e "$sed_script" | $KUBECTL $KUBECTL_OPTIONS create -f -

