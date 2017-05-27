#!/bin/bash

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/../env.sh

c=create
if [ "$1" == "d" ]; then
	c=delete
fi

$KUBECTL $KUBECTL_OPTIONS $c -f gateway-service.yaml
$KUBECTL $KUBECTL_OPTIONS $c -f server.yaml
$KUBECTL $KUBECTL_OPTIONS $c -f grafana-service.yaml