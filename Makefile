.PHONY: clean all fmt vet lint build test install static
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

LDFLAGS := ${LDFLAGS} \
	-X $(PROJECT)/main.GITCOMMIT=${GITCOMMIT} \
	-X $(PROJECT)/main.VERSION=${VERSION} \

all: clean build fmt lint test vet install

build:
	@echo "+ $@"
	go build -tags "$(BUILDTAGS)" -ldflags "${LDFLAGS}" .

static:
	@echo "+ $@"
	CGO_ENABLED=1 go build -tags "$(BUILDTAGS) cgo static_build" \
		-ldflags "-w -extldflags -static ${LDFLAGS}" -o binctr .

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

clean:
	@echo "+ $@"
	@$(RM) binctr

install:
	@echo "+ $@"
	@go install .
