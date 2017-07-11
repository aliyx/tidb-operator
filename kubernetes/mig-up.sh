#!/bin/bash

set -e

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/env.sh

image="${REGISTRY}/migrator:${VERSION}"
sh=${M_S_HOST}
sP=${M_S_PORT}
su=${M_S_USER}
sp=${M_S_PASSWORD}
db=${M_S_DB}

dh=${M_D_HOST}
dP=${M_D_PORT}
du=${M_D_USER}
dp=${M_D_PASSWORD}
api=${M_STAT_API}
op=${M_SYNC}

cell=`echo $CELL`
# Create the client service and replication controller.
sed_script=""
for var in cell image sh sP su sp db dh dP du dp api op; do
  sed_script+="s,{{$var}},${!var},g;"
done
echo sed_script
echo "Creating migration pod for $cell cell..."
cat migration-pod.yaml | sed -e "$sed_script" | $KUBECTL $KUBECTL_OPTIONS create -f -

