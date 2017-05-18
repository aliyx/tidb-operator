#!/bin/bash

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/../env.sh

$KUBECTL $KUBECTL_OPTIONS create -f gateway.yaml
$KUBECTL $KUBECTL_OPTIONS create -f server.yaml
$KUBECTL $KUBECTL_OPTIONS create -f grafana.yaml