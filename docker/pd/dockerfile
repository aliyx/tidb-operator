FROM golang:1.8

MAINTAINER yangxin45

ARG VERSION

RUN mkdir -p src/github.com/pingcap && \
    cd src/github.com/pingcap && \
    curl -L https://github.com/pingcap/pd/archive/${VERSION}.tar.gz | tar -xz && \
    mv pd-${VERSION} pd && cd pd && \
    make && \
    cp -R ./bin/* /go/bin/ && \
    rm -rf ../pd

ADD getpods /go/src/getpods
RUN GOBIN=/go/bin go install getpods

ADD config.toml /etc/pd/config.toml

EXPOSE 2379 2380

CMD ["pd-server"]
