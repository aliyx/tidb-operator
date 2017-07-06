# This image is only meant to be built from within the build.sh script.
FROM centos:latest

MAINTAINER yangxin45

# Copy binaries (placed by build.sh)
COPY base/tidb-server /

EXPOSE 4000

CMD ["/tidb-server"]