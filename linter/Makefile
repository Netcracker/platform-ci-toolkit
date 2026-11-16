IMAGE_NAME ?= ghcr.io/netcracker/linter
IMAGE_TAG ?= latest

.PHONY: build push lint

build:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

push:
	docker push $(IMAGE_NAME):$(IMAGE_TAG)

lint: build
	docker run --rm \
		-e RUN_LOCAL=true \
		-e VALIDATE_ALL_CODEBASE=true \
		-v $(CURDIR)/..:/tmp/lint \
		-w /tmp/lint \
		$(IMAGE_NAME):$(IMAGE_TAG)
