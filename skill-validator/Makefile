# skillcheck build. Requires the Go toolchain (see go.mod for the version).

VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)
PKG     := ./cmd/skillcheck
DIST    := dist
PLATFORMS := linux/amd64 linux/arm64 darwin/arm64 windows/amd64

.PHONY: build build-all test lint clean

# Build for the host platform into dist/skillcheck.
build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/skillcheck$(shell go env GOEXE) $(PKG)

# Build static executables for every supported platform into dist/.
build-all:
	@set -e; for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; ext=; [ "$$os" = windows ] && ext=.exe; \
		out=$(DIST)/skillcheck-$$os-$$arch$$ext; \
		echo "building $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o $$out $(PKG); \
	done

test:
	go test ./...

lint:
	go vet ./...
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed on:"; gofmt -l .; exit 1; }

clean:
	rm -rf $(DIST)
