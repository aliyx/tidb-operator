#!/bin/bash

set -x

db=$SRC_DB
if [ -z "$db" ]
then
    echo 'not set variable "SRC_DB" in env'
    exit 1
fi

# global var
retval=-1

sync_stat() {
    api=$API_STAT
    if [ -z "$api" ]; then
        echo "no set stat api in env"
        return 
    fi
    data="{\"type\":\"migrate\",\"status\":\"$1\"}"
    curl -X PATCH --connect-timeout 3 --header "Content-Type: application/json" -d "$data" $api
    if [ ! "$?" == 0 ]; then
        exit 1
    fi
}

# dump mysql data to local
dump() {
    # read env
    h=$SRC_HOST
    P=$SRC_PORT
    u=$SRC_USER
    p=$SRC_PASSWORD
    db=$SRC_DB
    if [ -z "$h" -o -z "$P" -o -z "$u" -o -z "$p" -o -z "$db" ] 
    then
        echo >&2 "Some mysql properites are not set..."
        retval=1
        return
    fi
    /usr/local/mydumper-linux-amd64/bin/mydumper -h $h -P $P -u $u -p $p -t 16 -F 128 -B $db --no-views --skip-tz-utc --no-locks -o /tmp/$db
    retval=$?
}

# load local data to tidb
load() {
    h=$DEST_HOST
    P=$DEST_PORT
    u=$DEST_USER
    p=$DEST_PASSWORD
    db=$SRC_DB
    if [ -z "$h" -o -z "$P" -o -z "$u" -o -z "$p" -o -z "$db" ] 
    then
        echo >&2 "Some tidb properites are not set..."
        retval=1
        return
    fi
    /usr/local/tidb-tools-latest-linux-amd64/bin/loader -h $h -P $P -u $u -p $p -t 4 -checkpoint=/tmp/$db/loader.checkpoint -d /tmp/$db
    retval=$?
}

sync_config="/tmp/$db/config.toml"
sync_meta="/tmp/$db/syncer.meta"

init_sync() {
    # sync config
    if ! [ -f $sync_config ]
    then
tee > $sync_config <<- EOF
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
    if ! [ -f $sync_meta ]
    then
        # set sync position
        dump_meta="/tmp/$db/metadata"
        binlog_name=$(cat $dump_meta |grep 'Log: ' | awk '{ print $2}')
        binlog_pos=$(cat $dump_meta |grep 'Pos: ' | awk '{ print $2}')
tee > $sync_meta <<- EOF
binlog-name = "$binlog_name"
binlog-pos = $binlog_pos
EOF
    fi
    /usr/local/tidb-tools-latest-linux-amd64/bin/syncer -config $sync_config
}

errHandle() {
     if [ ! "$retval" == 0 ]; then
        sync_stat Error
        exit 1
    fi
}

cmd=$1
if [ -z "$cmd" ]
then
    rm -rf /tmp/$db
    sync_stat Dumping
    dump
    errHandle
    sync_stat Loading
    load
    errHandle
    sync_stat Finished
elif [ "$cmd" == "dump" ]
then
    sync_stat Dumping
    dump
    errHandle
    sync_stat Dumped
elif [ "$cmd" == "load" ]
then
    sync_stat Loading
    load
    errHandle
    sync_stat Loaded
elif [ "$cmd" == "sync" ]
then
    rm -rf /tmp/$db
    sync_stat Dumping
    dump
   errHandle
    sync_stat Loading
    load
    errHandle
    sync_stat Syncing
    init_sync
    sync
fi