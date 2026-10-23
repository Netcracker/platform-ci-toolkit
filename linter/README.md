# Common Linter

This directory builds the common linter image. The image extends [GitHub Super-Linter](https://github.com/super-linter/super-linter) with organization-wide defaults and supports per-module Go linting for repositories that contain multiple Go modules.

## Build

```bash
make build
```

## Push

```bash
make push
```

## CI build

The image is built and published by the repository's `CI: Dev Docker Build` workflow, configured in `.qubership/docker.cfg`. The workflow builds the `linter` component from `linter/Dockerfile` and pushes it to the GitHub Container Registry (`ghcr.io`).

## Run locally

From a consumer repository that contains the consumer `Makefile`:

```bash
make lint                                   # Run Super-Linter on changed files
make lint FILTER=<regex>                    # Run Super-Linter on a filtered set of files
make lint-fix                               # Run Super-Linter with auto-fix enabled
make verify                                 # Run lint and tests
make verify FILTER=<regex>                  # Run lint and tests on a filtered set of files/modules
make test                                   # Run Go and Python tests (host or dedicated CI image, not the linter container)
make test-go                                # Auto-detect Go modules and run go test ./... where tests exist
make test-python                            # Placeholder; override in the consumer Makefile
```

For full usage instructions, including CI and GitHub Actions examples, see `docs/usage.md`.
See `templates/Makefile.consumer` for the consumer template, `docs/usage.md` for usage instructions, and `super-linter.env` for baseline environment variables.
