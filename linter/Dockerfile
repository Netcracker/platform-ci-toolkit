ARG SUPER_LINTER_VERSION=slim-v9
# Pin skillcheck to a public GHCR digest for reproducible builds.
ARG SKILLCHECK_IMAGE=ghcr.io/netcracker/skillcheck:common_linter_integration@sha256:2a0dcf5e80ebee88aae3cf5c2f9c4425a5cc9ba04da81d32ede265bdf7fffac5

FROM ${SKILLCHECK_IMAGE} AS skillcheck

FROM ghcr.io/super-linter/super-linter:${SUPER_LINTER_VERSION}

LABEL org.opencontainers.image.source="https://github.com/netcracker/platform-ci-toolkit"
LABEL org.opencontainers.image.description="Common linter image based on Super-Linter with skillcheck"

# Install make so that consumer Makefiles can be invoked both locally and in CI.
USER root
RUN if command -v apk >/dev/null 2>&1; then \
        apk add --no-cache make; \
    elif command -v apt-get >/dev/null 2>&1; then \
        apt-get update && apt-get install -y --no-install-recommends make && rm -rf /var/lib/apt/lists/*; \
    else \
        echo "No supported package manager found; please install make manually" && exit 1; \
    fi

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
