GO ?= go
DOCKER ?= docker
IMAGE ?= ghcr.io/adityathebe/restic-exporter
TAG ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo latest)
NAME ?= restic-exporter

ifeq ($(VERSION),)
  VERSION_TAG=$(shell git describe --abbrev=0 --tags --exact-match 2>/dev/null || echo latest)
else
  VERSION_TAG=$(VERSION)
endif

.PHONY: all build docker

all: build

build:
	mkdir -p bin
	$(GO) build -o bin/restic-exporter ./src

docker:
	$(DOCKER) build -t $(IMAGE):$(TAG) .

.PHONY: linux
linux:
	GOOS=linux GOARCH=amd64 go build  -o ./bin/$(NAME)_linux_amd64 -ldflags "-X \"main.version=$(VERSION_TAG)\""  ./src
	GOOS=linux GOARCH=arm64 go build  -o ./bin/$(NAME)_linux_arm64 -ldflags "-X \"main.version=$(VERSION_TAG)\""  ./src

.PHONY: darwin
darwin:
	GOOS=darwin GOARCH=amd64 go build -o ./bin/$(NAME)_darwin_amd64 -ldflags "-X \"main.version=$(VERSION_TAG)\""  ./src
	GOOS=darwin GOARCH=arm64 go build -o ./bin/$(NAME)_darwin_arm64 -ldflags "-X \"main.version=$(VERSION_TAG)\""  ./src

.PHONY: binaries
binaries: linux darwin

.PHONY: release
release: binaries
	mkdir -p .release
	cp bin/restic-exporter* .release/