#!/bin/bash

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/../env.sh

c=create
if [ "$1" == "d" ]; then
	c=delete
fi

cIp=''

$KUBECTL $KUBECTL_OPTIONS $c -f gateway.yaml

$KUBECTL $KUBECTL_OPTIONS $c -f server.yaml
if [ "$c" == "create" ]; then
	cIp=$($KUBECTL $KUBECTL_OPTIONS get -o template --template '{{.spec.clusterIP}}'  service prom-server)
	wait_for_complete $(echo "http://$cIp:9090/status")
fi

$KUBECTL $KUBECTL_OPTIONS $c -f grafana-service.yaml
if [ "$c" == "create" ]; then
	cIp=$($KUBECTL $KUBECTL_OPTIONS get -o template --template '{{.spec.clusterIP}}'  service grafana)
	wait_for_complete $(echo "http://$cIp:3000/datasources")
fi

if ! [ "$c" == "create" ]; then
  exit 0
fi

# import datasource
for file in *-datasource.json ; do
  if [ -e "$file" ] ; then
    echo "importing $file" &&
    curl --silent --fail --show-error \
      --request POST http://admin:admin@$cIp:3000/api/datasources \
      --header "Content-Type: application/json" \
      --data-binary "@$file" ;
    echo "" ;
  fi
done ;

# import dashboards
files=./grafana/*
for file in $files ; do
  if [ -e "$file" ] ; then
    echo "importing $file" &&
    ( echo '{"dashboard":'; \
      cat "$file"; \
      echo ',"overwrite":true,"inputs":[{"name":"DS_TIDB","type":"datasource","pluginId":"prometheus","value":"tidb"}]}' ) \
    | jq -c '.' \
    | curl --silent --fail --show-error \
      --request POST http://admin:admin@$cIp:3000/api/dashboards/import \
      --header "Content-Type: application/json" \
      --data-binary "@-" ;
    echo "" ;
  fi
done