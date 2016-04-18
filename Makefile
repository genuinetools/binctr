.PHONY: clean clean-rootfs all fmt vet lint build test install image.tar rootfs.go static
PREFIX?=$(shell pwd)
BUILDTAGS=seccomp apparmor

PROJECT := github.com/jfrazelle/binctr
VENDOR := vendor

# Variable to get the current version.
VERSION := $(shell cat VERSION)

# Variable to set the current git commit.
GITCOMMIT := $(shell git rev-parse --short HEAD)
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
GITUNTRACKEDCHANGES := $(shell git status --porcelain --untracked-files=no)
ifneq ($(GITUNTRACKEDCHANGES),)
GITCOMMIT := $(GITCOMMIT)-dirty
endif

IMAGE := alpine
DOCKER_ROOTFS_IMAGE := $(IMAGE)

LDFLAGS := ${LDFLAGS} \
	-X main.GITCOMMIT=${GITCOMMIT} \
	-X main.VERSION=${VERSION} \
	-X main.IMAGE=$(notdir $(IMAGE)) \
	-X main.IMAGESHA=$(shell docker inspect --format "{{.Id}}" $(IMAGE))

BINDIR := $(CURDIR)/bin

all: clean static fmt lint test vet install

build: rootfs.go
	@echo "+ $@"
	go build -tags "$(BUILDTAGS)" -ldflags "${LDFLAGS}" .

$(BINDIR):
	@mkdir -p $@

static: $(BINDIR) rootfs.go
	@echo "+ $@"
	CGO_ENABLED=1 go build -tags "$(BUILDTAGS) cgo static_build" \
		-ldflags "-w -extldflags -static ${LDFLAGS}" -o bin/$(notdir $(IMAGE)) .
	@echo "Static container created at: ./bin/$(notdir $(IMAGE))"
	@echo "Run with ./bin/$(notdir $(IMAGE))"

image.tar:
	docker pull --disable-content-trust=false $(DOCKER_ROOTFS_IMAGE)
	docker export $(shell docker create $(DOCKER_ROOTFS_IMAGE) sh) > $@

rootfs.go: image.tar
	GOMAXPROCS=1 go generate

fmt:
	@echo "+ $@"
	@gofmt -s -l . | grep -v $(VENDOR) | tee /dev/stderr

lint:
	@echo "+ $@"
	@golint ./... | grep -v $(VENDOR) | tee /dev/stderr

test: fmt lint vet
	@echo "+ $@"
	@go test -v -tags "$(BUILDTAGS) cgo" $(shell go list ./... | grep -v $(VENDOR))

vet:
	@echo "+ $@"
	@go vet $(shell go list ./... | grep -v $(VENDOR))

clean-rootfs:
	@sudo $(RM) -r rootfs

clean: clean-rootfs
	@echo "+ $@"
	@$(RM) binctr
	@$(RM) *.tar
	@$(RM) rootfs.go
	@$(RM) -r $(BINDIR)
	-@docker rm $(shell docker ps -aq) /dev/null 2>&1

install:
	@echo "+ $@"
	@go install .
