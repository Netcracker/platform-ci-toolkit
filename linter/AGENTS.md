# Common Linter

This directory builds the common linter image. The image extends GitHub Super-Linter v9 with organization-wide defaults. Keep `README.md` and `docs/usage.md` in sync with behavior changes.

## Commands

Run from `linter/`:

```sh
make build                 # build the local linter image
make push                  # push the image to the registry
make lint                  # run the linter against the parent repository
```

## Layout

- `Dockerfile`: installs `make` on top of the official Super-Linter image, copies the `skillcheck` binary from the public `ghcr.io/netcracker/skillcheck` image (pinned by digest), sets `GOMODCACHE=/tmp/lint/.go/pkg/mod` so the Go module cache lives inside the repository, and copies baseline rules plus `run.sh`.
- `run.sh`: container entry point. Ensures `/tmp/lint` points to the repository (linking to `CI_PROJECT_DIR` or `GITHUB_WORKSPACE` when running in CI), copies baseline linter configs to a writable directory, overlays consumer rules from `/tmp/lint/prechecks/custom-liners/`, loads environment files, runs Super-Linter, and runs `skillcheck` when `SKILLCHECK_PATTERNS` is configured.
- `super-linter.env`: baseline environment for Super-Linter. Disables languages that are not commonly used; excludes the Go module cache (`^\.go/pkg/mod/.*`) from linting; Go is left enabled so Super-Linter runs `GO` / `GO_MODULES`.
- `prechecks/base-linters/`: default linter rule files.
- `templates/Makefile.consumer`: full standalone consumer Makefile. Copy it into consumer repositories. It provides `verify`, `test`, `test-go`, and `test-python` targets. `lint` delegates to `lint-super-linter` and `skill-validate`; `verify` runs lint and tests; `test-go` auto-detects Go modules and runs tests in every module that has tests; `test-go` and `test-python` skip when no relevant files changed against `DEFAULT_BRANCH`. `FILTER` is passed only to Super-Linter. In CI it links `/tmp/lint` to the checkout so the image's `GOMODCACHE` resolves inside the repository.
- `docs/`: `usage.md` explains how to run the linter locally, in CI, and in GitHub Actions, including test and verify targets.

## Rules that are not obvious from the code

- **Go module cache lives inside the repository.** The image sets `GOMODCACHE=/tmp/lint/.go/pkg/mod` and `templates/Makefile.consumer` adjusts it to the actual checkout path in CI. This lets CI systems save and restore the cache while keeping it out of the Super-Linter scan via `FILTER_REGEX_EXCLUDE`. Super-Linter’s own `GO_MODULES` still relies on the normal Go download behavior inside the container.
- **Super-Linter plus skillcheck.** `make lint` runs Super-Linter and, when `SKILLCHECK_PATTERNS` is set, also runs `skillcheck`.
- **Set `FILTER_REGEX_INCLUDE` to run the full Super-Linter suite on a filtered set of files.** Set `FILTER_REGEX_INCLUDE` to a regex relative to the repo root, for example `make lint FILTER_REGEX_INCLUDE=services/backuper-service/.*`.
- **Super-Linter `RUN_LOCAL=true` is required in CI too.** It only tells Super-Linter that it is not running inside GitHub Actions; GitLab CI and local runs both set it.
- **Image registry and component name:** the CI build workflow reads `.qubership/docker.cfg`. The component is named `linter`, so the published image is `ghcr.io/netcracker/linter`.
- **Entry point behavior:** `Dockerfile` sets `ENTRYPOINT ["/usr/local/bin/run.sh"]` and `CMD ["all"]`. CI workflows disable the entry point with `--entrypoint=""` (GitHub Actions) or `entrypoint: [""]` (GitLab CI) so that `make lint` runs as the container command.
