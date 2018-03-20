# Set an output prefix, which is the local directory if not specified
PREFIX?=$(shell pwd)

# Setup name variables for the package/tool
NAME := binctr
PKG := github.com/genuinetools/$(NAME)

# Set any default go build tags
BUILDTAGS := seccomp apparmor

# Set the build dir, where built cross-compiled binaries will be output
BUILDDIR := ${PREFIX}/cross

IMAGE := alpine
IMAGE_DATA_FILE := image/bindata.go

# Populate version variables
# Add to compile time flags
VERSION := $(shell cat VERSION.txt)
GITCOMMIT := $(shell git rev-parse --short HEAD)
GITUNTRACKEDCHANGES := $(shell git status --porcelain --untracked-files=no)
ifneq ($(GITUNTRACKEDCHANGES),)
	GITCOMMIT := $(GITCOMMIT)-dirty
endif
CTIMEVAR=-X $(PKG)/version.GITCOMMIT=$(GITCOMMIT) -X $(PKG)/version.VERSION=$(VERSION) \
		 -X $(PKG)/image.NAME=$(notdir $(IMAGE)) \
		 -X $(PKG)/image.SHA=$(shell docker inspect --format "{{.Id}}" $(IMAGE))
GO_LDFLAGS=-ldflags "-w $(CTIMEVAR)"
GO_LDFLAGS_STATIC=-ldflags "-w $(CTIMEVAR) -extldflags -static"

all: clean build fmt lint test staticcheck vet ## Runs a clean, build, fmt, lint, test, staticcheck, and vet

.PHONY: build
build: $(BUILDDIR)/$(notdir $(IMAGE)) ## Builds a static executable or package

$(BUILDDIR):
	@mkdir -p $@

$(BUILDDIR)/$(notdir $(IMAGE)): $(BUILDDIR) $(IMAGE_DATA_FILE) *.go VERSION.txt
	@echo "+ $@"
	CGO_ENABLED=1 go build \
				-tags "$(BUILDTAGS) static_build" \
				${GO_LDFLAGS_STATIC} -o $@ .
	@echo "Static container for $(IMAGE) created at: $@"

.PHONY: fmt
fmt: ## Verifies all files have men `gofmt`ed
	@echo "+ $@"
	@gofmt -s -l . | grep -v '.pb.go:' | grep -v vendor | grep -v bindata.go | tee /dev/stderr

.PHONY: lint
lint: ## Verifies `golint` passes
	@echo "+ $@"
	@golint ./... | grep -v '.pb.go:' | grep -v vendor | tee /dev/stderr

.PHONY: test
test: ## Runs the go tests
	@echo "+ $@"
	@go test -v -tags "$(BUILDTAGS) cgo" $(shell go list ./... | grep -v vendor)

.PHONY: vet
vet: ## Verifies `go vet` passes
	@echo "+ $@"
	@go vet $(shell go list ./... | grep -v vendor) | grep -v '.pb.go:' | tee /dev/stderr

.PHONY: staticcheck
staticcheck: ## Verifies `staticcheck` passes
	@echo "+ $@"
	@staticcheck $(shell go list ./... | grep -v vendor) | grep -v '.pb.go:' | tee /dev/stderr

.PHONY: cover
cover: ## Runs go test with coverage
	@echo "" > coverage.txt
	@for d in $(shell go list ./... | grep -v vendor); do \
		go test -race -coverprofile=profile.out -covermode=atomic "$$d"; \
		if [ -f profile.out ]; then \
			cat profile.out >> coverage.txt; \
			rm profile.out; \
		fi; \
	done;

.PHONY: bump-version
BUMP := patch
bump-version: ## Bump the version in the version file. Set BUMP to [ patch | major | minor ]
	@go get -u github.com/jessfraz/junk/sembump # update sembump tool
	$(eval NEW_VERSION = $(shell sembump --kind $(BUMP) $(VERSION)))
	@echo "Bumping VERSION.txt from $(VERSION) to $(NEW_VERSION)"
	echo $(NEW_VERSION) > VERSION.txt
	@echo "Updating links to download binaries in README.md"
	sed -i s/$(VERSION)/$(NEW_VERSION)/g README.md
	git add VERSION.txt README.md
	git commit -vsam "Bump version to $(NEW_VERSION)"
	@echo "Run make tag to create and push the tag for new version $(NEW_VERSION)"

.PHONY: tag
tag: ## Create a new git tag to prepare to build a release
	git tag -sa $(VERSION) -m "$(VERSION)"
	@echo "Run git push origin $(VERSION) to push your new tag to GitHub and trigger a travis build."

.PHONY: image.tar
image.tar:
	docker pull --disable-content-trust=false $(IMAGE)
	docker export $(shell docker create $(IMAGE) sh) > $@

.PHONY: $(IMAGE_DATA_FILE)
$(IMAGE_DATA_FILE): image.tar
	GOMAXPROCS=1 go generate

.PHONY: clean
clean: ## Cleanup any build binaries or packages
	@echo "+ $@"
	$(RM) $(NAME)
	$(RM) -r $(BUILDDIR)
	@sudo $(RM) -r rootfs
	$(RM) *.tar
	$(RM) $(IMAGE_DATA_FILE)
	-@docker rm $(shell docker ps -aq) /dev/null 2>&1

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
