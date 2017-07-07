FROM golang:1.8

MAINTAINER yangxin45

ARG VERSION

RUN mkdir -p src/github.com/pingcap && \
    cd src/github.com/pingcap && \
    curl -L https://github.com/pingcap/tidb/archive/${VERSION}.tar.gz | tar -xz && \
    mv tidb-${VERSION} tidb && cd tidb && \
    make && \
    cp -f ./bin/tidb-server /tidb-server && \
    make clean

EXPOSE 4000

CMD ["/tidb-server"]