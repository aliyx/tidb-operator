# 
#FROM pingcap/tikv:region
FROM pingcap/rust

MAINTAINER yangxin45

ADD config.toml /etc/tikv/config.toml

ADD https://github.com/pingcap/tikv/archive/rc3.tar.gz /tikv-rc3.tar.gz

RUN tar xf tikv-rc3.tar.gz && \
    cd tikv-rc3 && \
    make release && \
    cp -f bin/tikv-server /tikv-server && \
    cd .. && \
    rm -rf tikv-rc3.tar.gz tikv-rc3

EXPOSE 20160

CMD ["/tikv-server"]