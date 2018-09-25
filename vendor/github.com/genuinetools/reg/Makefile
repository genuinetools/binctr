# Setup name variables for the package/tool
NAME := reg
PKG := github.com/genuinetools/$(NAME)

CGO_ENABLED := 0

# Set any default go build tags.
BUILDTAGS :=

include basic.mk

.PHONY: prebuild
prebuild:

.PHONY: dind
dind: stop-dind ## Starts a docker-in-docker container for running the tests with.
	docker run -d  \
		--name $(NAME)-dind \
		--privileged \
		-v $(CURDIR)/.certs:/etc/docker/ssl \
		-v $(CURDIR):/go/src/github.com/genuinetools/reg \
		-v /tmp:/tmp \
		$(REGISTRY)/docker:userns \
		dockerd -D --storage-driver $(DOCKER_GRAPHDRIVER) \
		-H tcp://127.0.0.1:2375 \
		--host=unix:///var/run/docker.sock \
		--exec-opt=native.cgroupdriver=cgroupfs \
		--insecure-registry localhost:5000 \
		--tlsverify \
		--tlscacert=/etc/docker/ssl/cacert.pem \
		--tlskey=/etc/docker/ssl/server.key \
		--tlscert=/etc/docker/ssl/server.cert

.PHONY: stop-dind
stop-dind: ## Stops the docker-in-docker container.
	@docker rm -f $(NAME)-dind >/dev/null 2>&1 || true

.PHONY: image-dev
image-dev:
	docker build --rm --force-rm -f Dockerfile.dev -t $(REGISTRY)/$(NAME):dev .

.PHONY: dtest
dtest: image-dev ## Run the tests in a docker container.
	docker run --rm -i $(DOCKER_FLAGS) \
		-v $(CURDIR):/go/src/github.com/genuinetools/reg \
		--workdir /go/src/github.com/genuinetools/reg \
		-v $(CURDIR)/.certs:/etc/docker/ssl:ro \
		-v /tmp:/tmp \
		--disable-content-trust=true \
		--net container:$(NAME)-dind \
		-e DOCKER_HOST=tcp://127.0.0.1:2375 \
		-e DOCKER_TLS_VERIFY=true \
		-e DOCKER_CERT_PATH=/etc/docker/ssl \
		-e DOCKER_API_VERSION \
		$(REGISTRY)/$(NAME):dev \
		make test

.PHONY: snakeoil
snakeoil: ## Update snakeoil certs for testing.
	go run /usr/local/go/src/crypto/tls/generate_cert.go --host localhost,127.0.0.1 --ca
	mv $(CURDIR)/key.pem $(CURDIR)/testutils/snakeoil/key.pem
	mv $(CURDIR)/cert.pem $(CURDIR)/testutils/snakeoil/cert.pem
