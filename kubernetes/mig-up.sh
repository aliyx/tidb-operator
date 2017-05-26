#!/bin/bash

set -e

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/env.sh

image="${REGISTRY}/migration:${VERSION}"
sh=${M_SRC_HOST}
sP=${M_SRC_PORT}
su=${M_SRC_USER}
sp=${M_SRC_PASSWORD}
db=${M_SRC_DB}

dh=${M_DEST_HOST}
dP=${M_DEST_PORT}
duser=${M_DEST_USER}
dp=${M_DEST_PASSWORD}
api=${M_STAT_API}
sync=${M_SYNC}

cell=`echo $CELL`
# Create the client service and replication controller.
sed_script=""
for var in cell image sh sP su sp db dh dP duser dp api sync; do
  sed_script+="s,{{$var}},${!var},g;"
done
echo sed_script
echo "Creating migration pod for $cell cell..."
cat migration-pod.yaml | sed -e "$sed_script" | $KUBECTL $KUBECTL_OPTIONS create -f -

