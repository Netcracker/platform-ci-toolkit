ARG SUPER_LINTER_VERSION=slim-v9
# Pin skillcheck to a public GHCR digest for reproducible builds.
ARG SKILLCHECK_IMAGE=ghcr.io/netcracker/skillcheck:common_linter_integration@sha256:2a0dcf5e80ebee88aae3cf5c2f9c4425a5cc9ba04da81d32ede265bdf7fffac5

# Go toolchain used by GO / GO_MODULES. The Super-Linter image ships an older
# Go, and consumers may require a newer one in go.mod (for example `go 1.27.2`).
# Go would then try to download that toolchain, which fails when the runner
# cannot reach sum.golang.org. Install the required toolchain here and pin
# GOTOOLCHAIN=local so nothing is ever downloaded at lint time.
ARG GO_VERSION=1.27.2
ARG GO_SHA256=ecbadb99091a3f46e31f5f934b068b1864eafa7995211b39eaddf76996045fe5

FROM ${SKILLCHECK_IMAGE} AS skillcheck

FROM ghcr.io/super-linter/super-linter:${SUPER_LINTER_VERSION}

# Re-declare the build arguments so they are in scope for the RUN instruction
# below (ARGs are only in scope within the stage where they are declared).
ARG GO_VERSION
ARG GO_SHA256

LABEL org.opencontainers.image.source="https://github.com/netcracker/platform-ci-toolkit"
LABEL org.opencontainers.image.description="Common linter image based on Super-Linter with skillcheck"

# Place the Go module cache inside the repository so CI systems can save and
# restore it. Consumers must set GOPROXY themselves if the default public proxy
# is unreachable or when private modules are used. CI jobs may override
# GOMODCACHE (for example to $CI_PROJECT_DIR/.go/pkg/mod).
ENV GOMODCACHE="/tmp/lint/.go/pkg/mod"

# Never download a toolchain at lint time; the one installed below is used.
ENV GOTOOLCHAIN=local

# Install make so CI can run `make lint` inside the linter container. Language
# unit tests (Go, Python, Java) run in language-specific test images, so no
# language runtimes or build tools are added here, except the pinned Go
# toolchain needed by GO_MODULES.
USER root
RUN if command -v apk >/dev/null 2>&1; then \
        apk add --no-cache make; \
    elif command -v apt-get >/dev/null 2>&1; then \
        apt-get update && apt-get install -y --no-install-recommends make ca-certificates curl && rm -rf /var/lib/apt/lists/*; \
    else \
        echo "No supported package manager found; please install make manually" && exit 1; \
    fi

# Install the pinned Go toolchain, replacing the one shipped in the base image.
RUN set -eux; \
    archive="go${GO_VERSION}.linux-amd64.tar.gz"; \
    curl -fsSL -o "/tmp/${archive}" "https://go.dev/dl/${archive}"; \
    echo "${GO_SHA256}  /tmp/${archive}" | sha256sum -c -; \
    rm -rf /usr/local/go; \
    tar -C /usr/local -xzf "/tmp/${archive}"; \
    rm -f "/tmp/${archive}"; \
    export PATH="/usr/local/go/bin:${PATH}"; \
    go version

ENV PATH="/usr/local/go/bin:${PATH}"

COPY --from=skillcheck /usr/local/bin/skillcheck /usr/local/bin/skillcheck
COPY prechecks/base-linters /etc/platform-ci-toolkit/base-liners
COPY super-linter.env /etc/platform-ci-toolkit/super-linter.env
COPY --chmod=755 run.sh /usr/local/bin/run.sh

# The entry point runs the common linter wrapper. CI pipelines disable the
# container entry point with --entrypoint="" and invoke `make lint` directly,
# so the consumer Makefile remains the single source of truth for both CI and
# local usage. The default CMD runs Super-Linter plus skillcheck.
ENTRYPOINT ["/usr/local/bin/run.sh"]
CMD ["all"]
