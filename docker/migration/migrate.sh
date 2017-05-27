#!/bin/bash

# set -ex

# global var
db=$M_S_DB
retval=-1

if [ -z "$db" ]
then
  echo -e "\033[31mnot set variable "M_S_DB" in env\033[0m"
  exit 1
fi

reset() {
  retval=-1
}

# Will try again when an error occurs
sync_migration_stat() {
  api=$M_STAT_API
  if [ -z "$api" ]; then
    echo -e >&2 "\033[33m[Warining]: no set stat api in env \033[0m"
    return 1
  fi
  data="{\"type\":\"migrate\",\"status\":\"$1\"}"
  for i in `seq 1 30`; 
  do
    curl -X PATCH --connect-timeout 3 --silent --output /dev/null --header "Content-Type: application/json" -d "$data" $api
    if [ ! "$?" == 0 ]; then
      echo -e >&2 "\033[31m[$(date)] sync migration status error, waiting for a maximum of 1 hour retry\033[0m"
      sleep $[i*3]
    else
      return 0
    fi
  done
  exit 1
}

# dump mysql data to local
dump() {
  # read env
  h=$M_S_HOST
  P=$M_S_PORT
  u=$M_S_USER
  p=$M_S_PASSWORD
  if [ -z "$h" -o -z "$P" -o -z "$u" -o -z "$p" ] 
  then
    echo -e >&2 "\033[31msome mysql properites are not set\033[0m"
    retval=1
    return $retval
  fi
  /usr/local/mydumper-linux-amd64/bin/mydumper -h $h -P $P -u $u -p $p -t 2 -F 128 -B $db --no-views --skip-tz-utc --no-locks -o /tmp/$db
  retval=$?
}

# load local data to tidb
load() {
  h=$M_D_HOST
  P=$M_D_PORT
  u=$M_D_USER
  p=$M_D_PASSWORD
  if [ -z "$h" -o -z "$P" -o -z "$u" -o -z "$p" ] 
  then
      echo -e >&2 "\033[31msome tidb properites are not set\033[0m"
      retval=1
      return $retval
  fi
  /usr/local/tidb-tools-latest-linux-amd64/bin/loader -h $h -P $P -u $u -p $p -t 4 -checkpoint=/tmp/$db/loader.checkpoint -d /tmp/$db
  retval=$?
}

syncer_config="/tmp/$db/config.toml"
syncer_meta="/tmp/$db/syncer.meta"

init_syncer() {
  # sync config
  if ! [ -f $syncer_config ]
  then
tee > $syncer_config <<- EOF
log-level = "info"
server-id = 101

meta = "$sync_meta"
worker-count = 1
batch = 1

[from]
host = "$SRC_HOST"
user = "$SRC_USER"
password = "$SRC_PASSWORD"
port = $SRC_PORT

[to]
host = "$DEST_HOST"
user = "$DEST_USER"
password = "$DEST_PASSWORD"
port = $DEST_PORT
EOF
  fi
}

sync() {
  echo "Starting incremental sync data"
  if ! [ -f $syncer_meta ]
  then
    # set sync position
    dump_meta="/tmp/$db/metadata"
    binlog_name=$(cat $dump_meta |grep 'Log: ' | awk '{ print $2}')
    binlog_pos=$(cat $dump_meta |grep 'Pos: ' | awk '{ print $2}')
tee > $syncer_meta <<- EOF
binlog-name = "$binlog_name"
binlog-pos = $binlog_pos
EOF
  fi
  /usr/local/tidb-tools-latest-linux-amd64/bin/syncer -config $syncer_config
}

err_handle() {
  if [ ! "$retval" == 0 ]; then
  sync_migration_stat $1
  exit 1
  fi
}

cmd=$1
if [ -z "$cmd" ]; then
  sync_migration_stat Dumping
  dump
  err_handle DumpError
  sync_migration_stat Loading
  load
  err_handle LoadError
  sync_migration_stat Finished
elif [ "$cmd" == "dump" ]; then
  sync_migration_stat Dumping
  dump
  err_handle DumpError
elif [ "$cmd" == "load" ]; then
  sync_migration_stat Loading
  dump
  err_handle LoadError
elif [ "$cmd" == "sync" ]; then
  sync_migration_stat Dumping
  dump
  err_handle DumpError
  sync_migration_stat Loading
  load
  err_handle LoadError
  sync_migration_stat Syncing
  init_syncer
  sync
fi