BINARY = vpp-monitoring-agent

.PHONY: glide format check-format check-vet build install clean

glide-setup:
	curl https://glide.sh/get | sh

glide:
	glide install

format:
	go fmt `go list ./... | grep -v '/vendor/'`

check-format:
	if [ -z "$$(gofmt -l `find . | grep \\.go$$ | grep -v '/vendor/'`)" ]; then \
	echo "Formatting is OK"; \
	else \
	echo "Formatting check failed for:"; \
	gofmt -l `find . | grep \\.go$$ | grep -v '/vendor/'`; \
	echo "Execute target: format"; \
	exit 1; \
	fi

check-vet:
	go vet `go list ./... | grep -v '/vendor/'`

check-gopath:
ifndef GOPATH
	$(error GOPATH is undefined)
endif

clean: check-gopath
	go clean .
	if [ -f $(GOPATH)/bin/$(BINARY) ] ; then rm $(GOPATH)/bin/$(BINARY) ; fi
	if [ -f $(GOPATH)/bin/$(BINARY)-configuration.yaml ] ; then rm $(GOPATH)/bin/$(BINARY)-configuration.yaml ; fi
	if [ -f $(GOPATH)/bin/$(BINARY).sh ] ; then rm $(GOPATH)/bin/$(BINARY).sh ; fi
	rm $(GOPATH)/bin/*.deb || true
	rm $(GOPATH)/bin/*.rpm || true

build: check-gopath check-format check-vet
	export GOARCH=amd64
	export GOOS=linux
	go build -o $(GOPATH)/bin/$(BINARY)

install: build
	go install
	cp configuration.yaml $(GOPATH)/bin/$(BINARY)-configuration.yaml
	cp $(BINARY).sh $(GOPATH)/bin/$(BINARY).sh

test:
	go test `go list ./... | grep -v /vendor/`

benchmark:
	go test -v -bench=. -benchtime=10s `go list ./... | grep -v /vendor/`

run:
	sudo $(GOPATH)/bin/$(BINARY).sh

full-build: glide clean install test

check-packaging-version:
ifndef BUILD_NUMBER
	$(error BUILD_NUMBER is undefined. Defined the env variable before reattemtping to build)
endif

deb-xenial-package: check-packaging-version
	./packaging/deb/xenial/debuild.sh
	mv ./packaging/deb/xenial/*.deb $(GOPATH)/bin

deb-trusty-package: check-packaging-version
	./packaging/deb/trusty/debuild.sh
	mv ./packaging/deb/trusty/*.deb $(GOPATH)/bin

rpm-package: check-packaging-version
	./packaging/rpm/rpmbuild.sh
	mv ./packaging/rpm/RPMS/x86_64/*.rpm $(GOPATH)/bin

