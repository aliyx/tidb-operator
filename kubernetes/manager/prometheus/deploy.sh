#!/bin/bash

usage="Usage: $(basename $0) -(c|d) -- manage prometheus, grafana monitoring system."

if [ "$1" == "-h" ]; then
	echo "$usage"
	exit 0
fi

script_root=$(dirname "${BASH_SOURCE}")
source $script_root/../../env.sh

c=''
if [ "$1" == "-c" ]; then
	c=create
elif [ "$1" == "-d" ]; then
	c=delete
else
	echo "$usage"
	exit 0
fi

cIp=''

$KUBECTL $KUBECTL_OPTIONS get pods > /dev/null 2>&1

if [[ "$?" != "0" ]]; then
  echo "can not access k8s cluster, maybe not at master node?"
else
  $KUBECTL $KUBECTL_OPTIONS $c -f gateway.yaml

  $KUBECTL $KUBECTL_OPTIONS $c -f server.yaml
  if [ "$c" == "create" ]; then
    cIp=$($KUBECTL $KUBECTL_OPTIONS get -o template --template '{{.spec.clusterIP}}' service prom-server)
    wait_for_complete $(echo "http://$cIp:9090/status")
  fi

  $KUBECTL $KUBECTL_OPTIONS $c -f grafana-service.yaml
  if [ "$c" == "create" ]; then
    cIp=$($KUBECTL $KUBECTL_OPTIONS get -o template --template '{{.spec.clusterIP}}' service grafana)
    wait_for_complete $(echo "http://$cIp:3000/datasources")
  fi
fi

if ! [ "$c" == "create" ]; then
	exit 0
fi

if [[ -z "$cIp" ]]; then
  cIp="10.213.44.128:12802"
else
  cIp=$cIp:3000
fi

# import datasource
for file in *-datasource.json; do
	if [ -e "$file" ]; then
		echo "importing $file" &&
			curl --silent --fail --show-error \
				--request POST http://admin:admin@$cIp/api/datasources \
				--header "Content-Type: application/json" \
				--data-binary "@$file"
		echo ""
	fi
done

set -e

pd='job=~\\"$tidb\\"'
tikv='instance=~\\"tikv-\$tidb-.*\\"'
tidb='instance=~\\"tidb-\$tidb-.*\\"'
# import dashboards
files=./grafana/*
for file in $files; do
	if [ -e "$file" ]; then
		echo "importing $file" &&
			(
				echo '{"dashboard":'
				cat "$file"
				echo ',"overwrite":true,"inputs":[{"name":"DS_TIDB-CLUSTER","type":"datasource","pluginId":"prometheus","value":"tidb-cluster"}]}'
			) | \
      # match metrics{}
      sed -e 's/\(pd[a-z_]*\){/\1{'"$pd"',/g' | \
      sed -e 's/\(tikv[a-z_]*\){/\1{'"$tikv"',/g' | \
      sed -e 's/\(tidb[a-z_]*\){/\1{'"$tidb"',/g' | \
      # match metrics[]
      sed -e 's/\(pd[a-z_]*\)\[/\1{'"$pd"'}[/g' | \
      sed -e 's/\(tikv[a-z_]*\)\[/\1{'"$tikv"'}[/g' | \
      sed -e 's/\(tidb[a-z_]*\)\[/\1{'"$tidb"'}[/g' | \
      # match metrics)
      sed -e 's/\(pd[a-z_]*\))/\1{'"$pd"'})/g' | \
      sed -e 's/\(tikv[a-z_]*\))/\1{'"$tikv"'})/g' | \
      sed -e 's/\(tidb[a-z_]*\))/\1{'"$tidb"'})/g' | \
      # remove instance=\"$instance\"
      sed -e 's/,instance=\\"$instance\\"//g' | \
			jq '.dashboard.templating={"list": [{"allValue": null,"current": {},"datasource": "${DS_TIDB-CLUSTER}","hide": 0,"includeAll": false,"label": null,"multi": false,"name": "tidb","options": [],"query": "label_values(pd_cluster_status, job)","refresh": 1,"regex": "","sort": 1,"tagValuesQuery": "","tags": [],"tagsQuery": "","type": "query","useTags": false}]}' |
			curl --silent --fail --show-error \
				--request POST http://admin:admin@$cIp/api/dashboards/import \
				--header "Content-Type: application/json" \
				--data-binary "@-"
		echo ""
	fi
done
