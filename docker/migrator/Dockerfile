FROM centos

MAINTAINER yangxin45

ENV PYTHON_PIP_VERSION 9.0.1

RUN set -ex; \
    \
    yum -y install wget; \
	\
	wget -O get-pip.py 'https://bootstrap.pypa.io/get-pip.py'; \
	\
	python get-pip.py \
		--disable-pip-version-check \
		--no-cache-dir \
		"pip==$PYTHON_PIP_VERSION" \
	; \
	pip --version; \
	rm -f get-pip.py

ENV PATH $PATH:/usr/src/migrator

RUN pip install requests

# https://download.pingcap.org/{?}
COPY ./tidb-enterprise-tools-latest-linux-amd64.tar.gz /usr/local/
COPY ./mydumper-linux-amd64.tar.gz /usr/local/

RUN cd /usr/local/ && \
    tar xzvf tidb-enterprise-tools-latest-linux-amd64.tar.gz && \
    tar xzvf mydumper-linux-amd64.tar.gz && \
    rm -rf *.tar.gz 

WORKDIR /usr/src/migrator

COPY ./migrator.sh ./migrator
COPY ./pkg ./pkg

CMD ["migrator"]