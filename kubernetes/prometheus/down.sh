#!/bin/bash

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/../env.sh

$KUBECTL $KUBECTL_OPTIONS delete service prom-server prom-gateway grafana
$KUBECTL $KUBECTL_OPTIONS delete rc prom-server prom-gateway grafana