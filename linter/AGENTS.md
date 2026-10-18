# Common Linter

This directory builds the Netcracker common linter image. The image extends GitHub Super-Linter v9 with organization-wide defaults. Go linting is handled by Super-Linter’s own `GO` / `GO_MODULES` checks; there is no separate custom Go linter. Keep `README.md` and `docs/linting.md` / `docs/super-linter-go-behavior.md` in sync with behavior changes.

## Commands

Run from `linter/`:

```sh
make build                 # build the local linter image
make push                  # push the image to the registry
make lint                  # run the linter against the parent repository
```

## Layout

- `Dockerfile`: installs `make` on top of the official Super-Linter image and copies baseline rules plus `run.sh`.
- `run.sh`: container entry point. Copies baseline linter configs to a writable directory, overlays consumer rules from `/tmp/lint/prechecks/custom-liners/`, loads environment files, and dispatches to Super-Linter.
- `super-linter.env`: baseline environment for Super-Linter. Disables languages that are not commonly used; Go is left enabled so Super-Linter runs `GO` / `GO_MODULES`.
- `prechecks/base-linters/`: default linter rule files.
- `templates/Makefile.consumer`: copy this into consumer repositories to call the linter both locally and in CI.
- `docs/`: `linting.md` describes current behavior; `super-linter-go-behavior.md` includes the full history and rejected alternatives.

## Rules that are not obvious from the code

- **No shared Go cache.** Earlier trials that mounted `.go-cache/` were removed to keep the integration simple and avoid accidentally linting cached dependency files. Super-Linter’s `GO_MODULES` relies on the normal Go download behavior inside the container.
- **Super-Linter only.** `make lint` runs Super-Linter; there is no separate custom Go linter step.
- **`make lint-filter` runs the full Super-Linter suite on a filtered set of files.** Set `FILTER` to a regex relative to the repo root, for example `make lint-filter FILTER=services/backuper-service/.*`.
- **Super-Linter `RUN_LOCAL=true` is required in CI too.** It only tells Super-Linter that it is not running inside GitHub Actions; GitLab CI and local runs both set it.
- **Image registry and component name:** the CI build workflow reads `.qubership/docker.cfg`. The component is named `linter`, so the published image is `ghcr.io/netcracker/linter`.
- **Entry point behavior:** `Dockerfile` sets `ENTRYPOINT ["/usr/local/bin/run.sh"]` and `CMD ["all"]`. CI workflows disable the entry point with `--entrypoint=""` (GitHub Actions) or `entrypoint: [""]` (GitLab CI) so that `make lint` runs as the container command.
