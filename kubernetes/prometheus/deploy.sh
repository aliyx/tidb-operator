#!/bin/bash

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/../env.sh

MAX_TASK_WAIT_RETRIES=${MAX_TASK_WAIT_RETRIES:-300}

function update_spinner_value () {
  spinner='-\|/'
  cur_spinner=${spinner:$(($1%${#spinner})):1}
}

function wait_for_complete () {
  url=$1
  response=0
  counter=0

  while [ $counter -lt $MAX_TASK_WAIT_RETRIES ]; do
    response=$(curl --write-out %{http_code} --silent --output /dev/null $url)
    echo -en "\r$url: waiting for return http_code:200..."
    if [ $response -eq 200 ]
    then
      echo Complete
      return 0
    fi
    update_spinner_value $counter
    echo -n $cur_spinner
    let counter=counter+1
    sleep 1
  done
  echo Timed out
  return -1
}

cmd=create
if [ "$1" == "d" ]; then
	cmd=delete
fi

$KUBECTL $KUBECTL_OPTIONS $c -f server.yaml

if [ "$cmd" == "create" ]; then
	cIp=$($KUBECTL $KUBECTL_OPTIONS get -o template --template '{{.spec.clusterIP}}'  service prom-server)
	wait_for_complete $(echo "http://$cIp:9090/api/v1/label/null/values")
fi

$KUBECTL $KUBECTL_OPTIONS $cmd -f gateway.yaml
$KUBECTL $KUBECTL_OPTIONS $cmd -f grafana-service.yaml