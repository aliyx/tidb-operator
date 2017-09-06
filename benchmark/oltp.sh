#!/bin/bash

# mysql -h10.213.44.128 -P14626 -utest -ptest test

# test insert
# prepare
sysbench oltp_insert \
--db-driver=mysql \
--mysql-host=10.213.44.128 --mysql-port=14626 --mysql-user=test --mysql-password=test --mysql-db=test \
--table_size=0 prepare

# run
sysbench oltp_insert \
--db-driver=mysql \
--mysql-host=10.213.44.128 --mysql-port=14626 --mysql-user=test --mysql-password=test --mysql-db=test \
--threads=50 --time=120 --report-interval=1  --table_size=1000000 run

# cleanup
sysbench oltp_insert \
--db-driver=mysql \
--mysql-host=10.213.44.128 --mysql-port=14626 --mysql-user=test --mysql-password=test --mysql-db=test \
--table_size=0 cleanup