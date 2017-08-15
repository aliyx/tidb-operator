ifneq ($(HTTPROXY),)
	DPROXY="--build-arg HTTP_PROXY=$(HTTPROXY) --build-arg HTTPS_PROXY=$(HTTPROXY)"
endif
export DPROXY

ifeq ($(REGISTRY),)
	REGISTRY = 10.209.224.13:10500/ffan/rds/
endif
export REGISTRY

ifeq ($(VERSION),)
	VERSION = latest
endif
export VERSION

null:
	@echo "Please specify a specific target!"

build: pd tikv tidb migrator prom-server tidb-gc tidb-operator
.PHONY: build

push: push-pd push-tikv push-tidb push-migrator push-prom-server push-tidb-gc push-tidb-operator
.PHONY: push

# uninstall apps from k8s
clean: clean-grafana clean-tidb-gc clean-tidb-operator
.PHONY: clean

# install apps to k8s
install: install-grafana install-tidb-gc install-tidb-operator
.PHONY: install

clean-grafana:
	cd kubernetes/prometheus; \
	./deploy.sh -d
.PHONY: clean-prometheus

clean-tidb-gc:
	cd kubernetes/manager; \
	./gc-down.sh
.PHONY: clean-tidb-gc

clean-tidb-operator:
	cd kubernetes/manager; \
	./op-down.sh
.PHONY: clean-tidb-operator

install-grafana:
	cd kubernetes/prometheus; \
	./deploy.sh -c
.PHONY: install-grafana

install-tidb-gc:
	cd kubernetes/manager; \
	./gc-up.sh
.PHONY: install-tidb-gc

install-tidb-operator:
	cd kubernetes/manager; \
	./op-up.sh
.PHONY: install-tidb-operator

migrator:
	cd docker/migrator; \
	./build.sh;
.PHONY: migrator

pd:
	cd docker/pd; \
	./build.sh;
.PHONY: pd

prom-server:
	cd docker/prom-server; \
	./build.sh;
.PHONY: prom-server

tidb:
	cd docker/tidb; \
	./build.sh;
.PHONY: tidb

tidb-gc:
	cd docker/tidb-gc; \
	./build.sh;
.PHONY: tidb-gc

tidb-operator:
	cd docker/tidb-operator; \
	./build.sh;
.PHONY: tidb-operator

tikv:
	cd docker/tikv; \
	./build.sh;
.PHONY: tikv

push-pd: docker/pd
	docker push $(REGISTRY)pd:$(VERSION)
.PHONY: push-pd

push-tikv:
	docker push $(REGISTRY)tikv:$(VERSION)
.PHONY: push-tikv

push-tidb:
	docker push $(REGISTRY)tidb:$(VERSION)
.PHONY: push-tidb

push-migrator: docker/migrator
	docker push $(REGISTRY)migrator:$(VERSION)
.PHONY: push-migrator

push-prom-server:
	docker push $(REGISTRY)prom-server:$(VERSION)
.PHONY: push-prom-server

push-tidb-gc:
	docker push $(REGISTRY)tidb-gc:$(VERSION)
.PHONY: push-tidb-gc

push-tidb-operator:
	docker push $(REGISTRY)tidb-operator:$(VERSION)
.PHONY: push-tidb-operator