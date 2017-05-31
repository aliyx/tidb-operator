FROM 1science/alpine:latest

MAINTAINER yangxin45

ADD migrate.sh /usr/local/bin/migrate

RUN cd /usr/local && \
    curl -L https://download.pingcap.org/tidb-tools-latest-linux-amd64.tar.gz | tar -xz && \
    curl -L https://download.pingcap.org/mydumper-linux-amd64.tar.gz | tar -xz && \
    rm -f tidb-tools-latest-linux-amd64.tar.gz mydumper-linux-amd64.tar.gz

CMD ["migrate"]