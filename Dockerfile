FROM golang:1.8

MAINTAINER yangxin45

ENV TIDB /go/src/github.com/ffan/tidb-operator
ENV PATH $TIDB:$PATH

COPY . $TIDB

# Compile the binary and statically link
RUN cd $TIDB && CGO_ENABLED=0 go build -ldflags '-d -w -s'

WORKDIR $TIDB

EXPOSE 12808

CMD ["tidb-operator"]