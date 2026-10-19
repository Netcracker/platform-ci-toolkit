# Common Linter

This directory builds the Netcracker common linter image. The image extends [GitHub Super-Linter](https://github.com/super-linter/super-linter) with organization-wide defaults and supports per-module Go linting for repositories that contain multiple Go modules.

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

From a consumer repository that contains the consumer `Makefile` template:

```bash
make lint                                   # Run Super-Linter on changed files
make lint-filter FILTER=<regex>             # Run Super-Linter on a filtered set of files
make lint-fix                               # Run Super-Linter with auto-fix enabled
```

For full usage instructions, including CI and GitHub Actions examples, see `docs/usage.md`.
See `templates/Makefile.consumer` for the template, `docs/linting.md` for Go linting details, and `super-linter.env` for baseline environment variables.
