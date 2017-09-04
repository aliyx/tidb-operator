#!/bin/bash

usage="Usage: `basename $0` -(c|d) -- manage prometheus, grafana monitoring system."

if [ "$1" == "-h" ]; then
  echo "$usage"
  exit 0
fi

script_root=`dirname "${BASH_SOURCE}"`
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

cIp='10.213.44.128:12802'

if ! [ "$c" == "create" ]; then
  exit 0
fi

# import dashboards
files=./grafana/cluster-tikv_rev8.json
for file in $files ; do
  if [ -e "$file" ] ; then
    echo "importing $file" &&
    ( echo '{"dashboard":'; \
      cat "./grafana/cluster-tikv_rev8.json"; \
      echo ',"overwrite":true,"timezone":"utc"}' ) \
    | jq -c '.' \
    | curl --silent --fail --show-error \
      --request POST http://admin:admin@$cIp/api/dashboards/import \
      --header "Content-Type: application/json" \
      --data-binary "@-" ;
    echo "" ;
  fi
done

