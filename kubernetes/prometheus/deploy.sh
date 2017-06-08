#!/bin/bash

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/../env.sh

c=create
if [ "$1" == "d" ]; then
	c=delete
fi

$KUBECTL $KUBECTL_OPTIONS $c -f server.yaml

if [ "$c" == "create" ]; then
	cIp=$($KUBECTL $KUBECTL_OPTIONS get -o template --template '{{.spec.clusterIP}}'  service prom-server)
	wait_for_complete $(echo "http://$cIp:9090/api/v1/label/null/values")
fi

$KUBECTL $KUBECTL_OPTIONS $c -f gateway.yaml
$KUBECTL $KUBECTL_OPTIONS $c -f grafana-service.yaml