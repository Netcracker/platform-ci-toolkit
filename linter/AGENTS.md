# Common Linter

This directory builds the Netcracker common linter image. The image extends GitHub Super-Linter v6 with organization-wide defaults and adds a custom per-module Go linter for repositories that contain multiple Go modules. Keep `README.md` and `docs/linting.md` / `docs/super-linter-go-behavior.md` in sync with behavior changes.

## Commands

Run from `linter/`:

```sh
make build                 # build the local linter image
make push                  # push the image to the registry
make lint                  # run the linter against the parent repository
```

## Layout

- `Dockerfile`: multi-stage-ish build that installs `make` on top of the official Super-Linter image and copies baseline rules plus `run.sh`.
- `run.sh`: container entry point. Copies baseline linter configs to a writable directory, overlays consumer rules from `/tmp/lint/prechecks/custom-liners/`, loads environment files, and dispatches to Super-Linter or the custom Go linter.
- `super-linter.env`: baseline environment for Super-Linter. Disables languages that are not commonly used and enables `PLATFORM_CI_CUSTOM_GO_LINT`.
- `prechecks/base-linters/`: default linter rule files.
- `templates/Makefile.consumer`: copy this into consumer repositories to call the linter both locally and in CI.
- `docs/`: `linting.md` describes current behavior; `super-linter-go-behavior.md` includes the full history and rejected alternatives.

## Rules that are not obvious from the code

- **No shared Go cache.** Earlier trials that mounted `.go-cache/` were removed to keep the integration simple and avoid accidentally linting cached dependency files. The per-module `golangci-lint` runs rely on the normal Go download behavior inside the container.
- **Custom Go linter is driven through the Makefile.** `run.sh` exposes `super-linter` and `go` commands; the consumer `Makefile` maps `make lint`, `make lint_go_modules`, `make super-linter`, `make lint-module`, and `make lint-fix` to them.
- **Super-Linter `RUN_LOCAL=true` is required in CI too.** It only tells Super-Linter that it is not running inside GitHub Actions; GitLab CI and local runs both set it.
- **Image registry and component name:** the CI build workflow reads `.qubership/docker.cfg`. The component is named `linter`, so the published image is `ghcr.io/netcracker/linter`.
- **Entry point behavior:** `Dockerfile` sets `ENTRYPOINT ["/usr/local/bin/run.sh"]` and `CMD ["all"]`. CI workflows disable the entry point with `--entrypoint=""` (GitHub Actions) or `entrypoint: [""]` (GitLab CI) so that `make lint` runs as the container command.
