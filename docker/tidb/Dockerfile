FROM golang:1.7

MAINTAINER yangxin45

RUN mkdir -p src/github.com/pingcap && \
    cd src/github.com/pingcap && \
    curl -sL https://github.com/pingcap/tidb/archive/rc3.tar.gz | tar -xz && \
    mv tidb-rc3 tidb && cd tidb && \
    make && \
    cp -f ./bin/tidb-server /tidb-server && \
    make clean

CMD ["/tidb-server"]