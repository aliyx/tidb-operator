#!/bin/bash

# set -x

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

fun

# global var
db=$M_S_DB
retval=-1

if [ -z "$db" ]; then
  echo -e "\033[31mnot set variable "M_S_DB" in env\033[0m"
  exit 1
fi

reset() {
  retval=-1
}

# Will try again when an error occurs
sync_migration_stat() {
  python -c 'import pkg.est; rest.sync_stat($1,$2)'
}

# dump mysql data to local
dump() {
  # read env
  h=$M_S_HOST
  P=$M_S_PORT
  u=$M_S_USER
  p=$M_S_PASSWORD
  if [ -z "$h" -o -z "$P" -o -z "$u" -o -z "$p" ]; then
    echo -e "\033[31msome mysql properites are not set\033[0m" >&2
    retval=1
    return $retval
  fi
  /usr/local/mydumper-linux-amd64/bin/mydumper -h $h -P $P -u $u -p $p -B $db \
    -t 2 -F 128 \
    --no-views --skip-tz-utc \
    --no-locks --less-locking --verbose 3 \
    -o /tmp/$db
  retval=$?
}

# load local data to tidb
load() {
  h=$M_D_HOST
  P=$M_D_PORT
  u=$M_D_USER
  p=$M_D_PASSWORD
  if [ -z "$h" -o -z "$P" -o -z "$u" -o -z "$p" ]; then
    echo -e "\033[31msome tidb properites are not set\033[0m" >&2
    retval=1
    return $retval
  fi
  /usr/local/tidb-enterprise-tools-latest-linux-amd64/bin/loader -h $h -P $P -u $u -p $p \
    -t 4 \
    -checkpoint=/tmp/$db/loader.checkpoint \
    -d /tmp/$db
  retval=$?
}

syncer_config="/tmp/$db/config.toml"
syncer_meta="/tmp/$db/syncer.meta"

init_syncer() {
  # sync config
  if ! [ -f $syncer_config ]; then
    tee >$syncer_config <<-EOF
log-level = "info"
server-id = 101

meta = "$syncer_meta"
worker-count = 1
batch = 1

[from]
host = "$M_S_HOST"
port = $M_S_PORT
user = "$M_S_USER"
password = "$M_S_PASSWORD"

[to]
host = "$M_D_HOST"
port = $M_D_PORT
user = "$M_D_USER"
password = "$M_D_PASSWORD"
EOF
  fi
}

sync() {
  if ! [ -f $syncer_meta ]; then
    # set sync position
    dump_meta="/tmp/$db/metadata"
    binlog_name=$(cat $dump_meta | grep 'Log: ' | head -1 | awk '{ print $2}')
    binlog_pos=$(cat $dump_meta | grep 'Pos: ' | head -1 | awk '{ print $2}')
    if [ -z "$binlog_name" -o -z "$binlog_pos" ]; then
      echo
      return 1
    fi
    echo "binlog_name: $binlog_name, binlog_pos: $binlog_pos"
tee >$syncer_meta <<-EOF
binlog-name = "$binlog_name"
binlog-pos = $binlog_pos
EOF
  fi
  killall -9 syncer
  /usr/local/tidb-enterprise-tools-latest-linux-amd64/bin/syncer -config $syncer_config
}

err_handle() {
  if [ ! "$retval" == 0 ]; then
    sync_migration_stat $1
    exit 1
  fi
}

cmd=$1
if [ -z "$cmd" ]; then
  echo -e "${GREEN}[$(date)] Start dumping the remote mysql data to the local server.${NC}"
  sync_migration_stat Dumping
  dump
  err_handle DumpError
  echo -e "${GREEN}[$(date)] Start loading local server data to tidb.${NC}"
  sync_migration_stat Loading
  load
  err_handle LoadError
  sync_migration_stat Finished
  echo "[$(date)] Finished."
elif [ "$cmd" == "dump" ]; then
  echo -e "${GREEN}[$(date)] Start dumping the remote mysql data to the local server.${NC}"
  sync_migration_stat Dumping
  dump
  err_handle DumpError
  echo -e "[$(date)] Finished."
elif [ "$cmd" == "load" ]; then
  echo -e "${GREEN}[$(date)] Start loading local server data to tidb.${NC}"
  sync_migration_stat Loading
  dump
  err_handle LoadError
  echo -e "[$(date)] Finished."
elif [ "$cmd" == "isync" ]; then
  echo -e "${GREEN}[$(date)] Start incremental syncing the remote mysql data to tidb...${NC}"
  sync_migration_stat Syncing
  init_syncer
  sync
  sync_migration_stat SyncError
  echo -e "[$(date)] Finished."
elif [ "$cmd" == "sync" ]; then
  echo -e "${GREEN}[$(date)] Start dumping the remote mysql data to the local server.${NC}"
  sync_migration_stat Dumping
  dump
  err_handle DumpError
  echo -e "${GREEN}[$(date)] Start loading local server data to tidb.${NC}"
  sync_migration_stat Loading
  load
  err_handle LoadError
  echo -e "${GREEN}[$(date)] Start incremental syncing the remote mysql data to tidb...${NC}"
  sync_migration_stat Syncing
  init_syncer
  sync
  sync_migration_stat SyncError
fi