#!/bin/bash

# This is an example script that creates a fully functional tidb cluster.
# It performs the following steps:
# 1. Create pd clusters
# 2. Create tikv clusters
# 3. Create tidb clusters

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/env.sh

pd_replicas=${PD_REPLICAS:-3}
kv_replicas=${KV_REPLICAS:-3}
db_replicas=${DB_REPLICAS:-2}

PD_TEMPLATE=${PD_TEMPLATE:-'pd-template.yaml'}
TIKV_TEMPLATE=${TIKV_TEMPLATE:-'tikv-pod.yaml'}
TIDB_TEMPLATE=${TIDB_TEMPLATE:-'tidb-template.yaml'}
MAX_TASK_WAIT_RETRIES=${MAX_TASK_WAIT_RETRIES:-300}

# export for other scripts
export NS=$NS

cell=`echo $CELL`

echo "****************************"
echo "*Creating tidb cluster namespace: $NS"
echo "*  Cell: $cell"
echo "*  PD count: $pd_replicas"
echo "*  Tikv count: $kv_replicas"
echo "*  Tidb count: $db_replicas"
echo "****************************"

# echo 'Running namespace-up.sh' && ./namespace-up.sh

echo 'Running pd-up.sh' && ./pd-up.sh
wait_for_running_tasks pd-$cell $pd_replicas
cIp=$($KUBECTL $KUBECTL_OPTIONS get -o template --template '{{.spec.clusterIP}}'  service pd-$cell)
wait_for_complete $(echo "http://$cIp:2379/pd/api/v1/leader")

echo 'Running tikv-up.sh' && ./tikv-up.sh
wait_for_running_tasks tikv-$cell $kv_replicas

echo 'Running tidb-up.sh' && ./tidb-up.sh
wait_for_running_tasks tidb-$cell $db_replicas

tidb_cluster=''
tidb_status_server=''
echo Geting tidb external port
tp=''
until [ $tp ]; do
  tp=`$KUBECTL $KUBECTL_OPTIONS get -o template --template '{{index (index .spec.ports 0) "nodePort"}}' service tidb-$cell`
  sleep 1
done
tsp=''
until [ $tsp ]; do
  tsp=`$KUBECTL $KUBECTL_OPTIONS get -o template --template '{{index (index .spec.ports 1) "nodePort"}}' service tidb-$cell`
  sleep 1
done
l_ip=$(/sbin/ifconfig eth0 | grep 'netmask ' | cut -d: -f2 | awk '{print $2}')
tidb_server="$l_ip:$tp"
tidb_status_server="$l_ip:$tsp/status"

echo "****************************"
echo "* Complete!"
echo "* tidb server: $tidb_server"
echo "* tidb status server: $tidb_status_server"
echo "****************************"
