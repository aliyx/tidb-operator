FROM centos:latest

MAINTAINER yangxin45

COPY migrate.sh /usr/local/bin/migrate

ADD https://download.pingcap.org/tidb-enterprise-tools-latest-linux-amd64.tar.gz /usr/local/tidb-enterprise-tools
ADD https://download.pingcap.org/mydumper-linux-amd64.tar.gz /usr/local/mydumper

CMD ["migrate"]