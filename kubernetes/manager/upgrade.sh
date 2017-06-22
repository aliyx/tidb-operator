#!/bin/bash

set -e

script_root=`dirname "${BASH_SOURCE}"`
source $script_root/../env.sh

c=""
if [ "$1" == "c" ]; then
  c=create
elif [ "$1" == "d" ]; then
  c=delete
else
  echo "Please specify the operation 'c' or 'd' "
  exit 1
fi

version=""
if [ -z "$2" ]; then
  echo "Please specify the version 'rc2','rc3'... "
  exit 1
fi
echo $c
echo $version
sed_script+="s,{{version}},${!var},g;"
cat upgrade-daemonset.yaml | sed -e "$sed_script" | $KUBECTL $KUBECTL_OPTIONS create -f -