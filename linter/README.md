# Common Linter

This directory builds the common linter image. The image extends [GitHub Super-Linter](https://github.com/super-linter/super-linter) with organization-wide defaults, supports per-module Go linting for repositories that contain multiple Go modules, and includes the `skillcheck` utility for validating Agent Skills.

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
make lint                                   # Run Super-Linter + skill validation (when SKILLCHECK_PATTERNS is set)
make lint-super-linter                      # Run Super-Linter only
make skill-validate                         # Run skillcheck only (skips when SKILLCHECK_PATTERNS is empty)
make lint FILTER_REGEX_INCLUDE=<regex>      # Run lint on a filtered set of files
make lint-fix                               # Run Super-Linter with auto-fix enabled
make verify                                 # Run lint and tests
make verify FILTER_REGEX_INCLUDE=<regex>    # Run lint on a filtered set of files; tests run for the changed language
make test                                   # Run Go and Python tests (host or dedicated CI image, not the linter container)
make test-go                                # Auto-detect Go modules and run go test ./... where tests exist
make test-python                            # Run Python tests (requires PY_BUILD)
```

For full usage instructions, including CI and GitHub Actions examples, see `docs/usage.md`.
See `templates/Makefile.consumer` for the consumer template, `docs/usage.md` for usage instructions, and `super-linter.env` for baseline environment variables.
