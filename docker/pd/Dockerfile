FROM golang:1.7

MAINTAINER yangxin45

RUN mkdir -p src/github.com/pingcap && \
    cd src/github.com/pingcap && \
    curl -L https://github.com/pingcap/pd/archive/rc2.tar.gz | tar -xz && \
    mv pd-rc2 pd && cd pd && \
    make && \
    cp -f ./bin/pd-server /go/bin/pd-server && \
    rm -rf ../pd

ADD getpods /go/src/getpods
ADD getid /go/src/getid
RUN GOBIN=/go/bin go install getpods
RUN GOBIN=/go/bin go install getid

ENV TZ=Asia/Shanghai

ADD config.toml /etc/pd/config.toml

CMD ["pd-server"]
