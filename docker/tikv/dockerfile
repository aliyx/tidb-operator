# ARG VERSION=latest
# FROM pingcap/tikv:$VERSION
FROM pingcap/tikv:pre-ga

ARG VERSION=latest

COPY config-${VERSION}.toml /etc/tikv/config.toml
COPY bin/mountpath /usr/local/bin/