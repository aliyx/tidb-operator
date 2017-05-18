FROM golang:1.7

MAINTAINER yangxin45

ADD getpods /go/src/getpods
ADD getid /go/src/getid
RUN GOBIN=/go/bin go install getpods
RUN GOBIN=/go/bin go install getid
# install etcdctl tool
# RUN mkdir -p src/github.com/coreos && \
#     cd src/github.com/coreos && \
#     curl -sL https://github.com/coreos/etcd/archive/v3.1.5.tar.gz | tar -xz && \
#     mv etcd-3.1.5 etcd && \
#     cp -R etcd/cmd/vendor /go/src/github.com/coreos/etcd/vendor && \
#     go install github.com/coreos/etcd/etcdctl && \
#     rm -rf etcd

ADD config.toml /etc/pd/config.toml

RUN mkdir -p src/github.com/pingcap && \
    cd src/github.com/pingcap && \
    curl -L https://github.com/pingcap/pd/archive/rc2.tar.gz | tar -xz && \
    mv pd-rc2 pd && cd pd && \
    make && \
    cp -f ./bin/pd-server /go/bin/pd-server && \
    rm -rf ../pd
CMD ["pd-server"]
