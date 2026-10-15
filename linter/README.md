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
make lint              # Run Super-Linter and the custom Go module linter
make lint_go_modules   # Run only the custom Go module linter
make lint-module MODULE=backuper-service
```

See `templates/Makefile.consumer` for the template, `docs/super-linter-go-behavior.md` for how Go linting works, and `super-linter.env` for baseline environment variables.
