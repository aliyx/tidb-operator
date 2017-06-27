FROM golang:1.8

MAINTAINER yangxin45

ENV TIDB $GOPATH/src/github.com/ffan/tidb-operator
RUN mkdir -p $TIDB

ADD . $TIDB

# Compile the binary and statically link
RUN cd $TIDB && CGO_ENABLED=0 go build -ldflags '-d -w -s'

ENV PATH $TIDB:$PATH
WORKDIR $TIDB

CMD ["tidb-operator"]