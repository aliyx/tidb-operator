FROM golang:1.8

MAINTAINER yangxin45

ENV TIDB_K8S $GOPATH/src/github.com/ffan/tidb-k8s
RUN mkdir -p $TIDB_K8S

ADD . $TIDB_K8S

# Compile the binary and statically link
RUN cd $TIDB_K8S && CGO_ENABLED=0 go build -ldflags '-d -w -s'

ENV PATH $TIDB_K8S:$PATH
WORKDIR $TIDB_K8S

CMD ["tidb-k8s"]