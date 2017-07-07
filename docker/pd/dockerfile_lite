# This image is only meant to be built from within the build.sh script.
FROM debian:jessie

# Copy binaries (placed by build.sh)
COPY base/* /usr/bin/

ADD config.toml /etc/pd/config.toml

EXPOSE 2379 2380

CMD ["pd-server"]