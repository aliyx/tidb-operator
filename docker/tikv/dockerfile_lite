# This image is only meant to be built from within the build.sh script.
FROM centos:latest

ARG VERSION=latest

# Copy binaries (placed by build.sh)
COPY base/tikv-server /

COPY config-${VERSION}.toml /etc/tikv/config.toml
COPY bin/mountpath /usr/local/bin/

EXPOSE 20160

CMD ["/tikv-server"]