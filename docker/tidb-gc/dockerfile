FROM golang:1.8

MAINTAINER yangxin45

COPY . /go/src/github.com/ffan/tidb-operator

# Compile the binary and statically link
RUN cd /go/src/github.com/ffan/tidb-operator/cmd/tidb-gc && CGO_ENABLED=0 go install -ldflags '-d -w -s'

CMD ["tidb-gc"]