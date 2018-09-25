# Setup name variables for the package/tool
NAME := binctr
PKG := github.com/genuinetools/$(NAME)

CGO_ENABLED := 1

# Set any default go build tags.
BUILDTAGS := seccomp apparmor

.PHONY: everything
everything: clean fmt lint test staticcheck vet alpine busybox cl-k8s ## Builds a static executable or package.

include basic.mk

.PHONY: prebuild
prebuild:

.PHONY: alpine
alpine:
	@echo "+ $@"
	go generate ./examples/$@/...
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build \
				-tags "$(BUILDTAGS) static_build" \
				${GO_LDFLAGS_STATIC} -o $@ ./examples/$@/...
	@echo "Static container for $@ created at: ./$@"

.PHONY: busybox
busybox:
	@echo "+ $@"
	go generate ./examples/$@/...
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build \
				-tags "$(BUILDTAGS) static_build" \
				${GO_LDFLAGS_STATIC} -o $@ ./examples/$@/...
	@echo "Static container for $@ created at: ./$@"

.PHONY: cl-k8s
cl-k8s:
	@echo "+ $@"
	go generate ./examples/$@/...
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build \
				-tags "$(BUILDTAGS) static_build" \
				${GO_LDFLAGS_STATIC} -o $@ ./examples/$@/...
	@echo "Static container for $@ created at: ./$@"
