# 
#FROM pingcap/tikv:region
FROM pingcap/rust

MAINTAINER yangxin45

ADD config.toml /etc/tikv/config.toml

RUN cd / && \
    curl -L https://github.com/pingcap/tikv/archive/rc3.tar.gz | tar -xz && \
    mv tikv-rc3 tikv && cd tikv && \
    cargo build --release && \
    cp -f target/release/tikv-server /tikv-server && \
    cargo clean

CMD ["/tikv-server"]