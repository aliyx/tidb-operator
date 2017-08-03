#!/bin/bash

usage="Usage: $(basename $0) -(c|d) {version} -- pull new pd/tikv/tidb image to each node."

if [ "$1" == "-h" ]; then
	echo "$usage"
	exit 0
fi

c=""
if [ "$1" == "-c" ]; then
	c=create
elif [ "$1" == "-d" ]; then
	c=delete
else
	echo "Please specify the operation '-c' or '-d'"
	exit 1
fi

version="$2"
if [ -z "$version" ]; then
	echo "Please specify the version, eg, 'rc3','latest' etc"
	exit 1
fi

set -e

script_root=$(dirname "${BASH_SOURCE}")
source $script_root/../env.sh

sed_script+="s,{{version}},${version},g;"
cat upgrade-daemonset.yaml | sed -e "$sed_script" | $KUBECTL $KUBECTL_OPTIONS $c -f -
