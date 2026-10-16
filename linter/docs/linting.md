# Go linting with the common linter

This document describes the current Go linting behavior of the platform-ci-toolkit common linter. For the full history, including trials and rejected alternatives, see [`super-linter-go-behavior.md`](super-linter-go-behavior.md).

## How it works

The common linter image is based on GitHub Super-Linter v9 and adds a custom Go module linter. The wrapper script `/usr/local/bin/run.sh` exposes three commands:

| Command | What it runs |
| --- | --- |
| `run.sh` (no argument) | Super-Linter for non-Go files, then the custom Go linter |
| `run.sh super-linter` | Super-Linter only |
| `run.sh go` | Custom per-module Go linter only |

Consumer repositories invoke these commands through the `Makefile` template so the same targets work both in CI pipelines and locally.

## Makefile targets

```bash
make lint                 # Super-Linter + custom Go linter
make lint_go_modules      # custom Go linter only
make lint-module MODULE=backuper-service  # custom Go linter for one module only
make lint-fix             # Super-Linter with auto-fix enabled
```

> **Note:** `make lint-module` runs **only** the custom Go linter for the requested module. It does not run Super-Linter (Markdown, YAML, JSON, Dockerfile, etc.). Use `make lint` when you need the full suite, or `make super-linter` for Super-Linter alone.

In CI, the reusable workflow runs inside the linter container with `--entrypoint=""`, `RUN_LOCAL=true`, and executes `make lint`. In local mode, the Makefile pulls the image and runs it via Docker. `RUN_LOCAL=true` is required whenever Super-Linter is not running inside GitHub Actions, so CI pipelines set it as well.

> **Note:** `CI=true` is not set manually in any file — GitLab CI and GitHub Actions runners set it automatically. The Makefiles simply check `ifeq ($(CI),true)` to choose the CI branch.

## What happens when you run `make lint`

### Local mode (`CI` is unset)

```
make lint
  └─ ensure-linter-image
       └─ docker run ghcr.io/netcracker/linter:latest
            └─ ENTRYPOINT /usr/local/bin/run.sh  (CMD defaults to "all")
                 └─ run.sh all
                      1. Copy base rules from /etc/platform-ci-toolkit/base-liners
                      2. Overlay /tmp/lint/prechecks/custom-liners/*
                      3. Load /etc/platform-ci-toolkit/super-linter.env
                      4. Load /tmp/lint/prechecks/custom-liners/super-linter.env
                      5. /action/lib/linter.sh   (Super-Linter)
                      6. lint_go_modules()       (custom per-module Go linter)
```

The repository root is mounted at `/tmp/lint`. `run.sh` writes the merged rule set to a temporary directory outside the mount so the working tree is never modified.

### CI mode (`CI=true`)

The CI runner starts a container from the linter image with the entry point disabled, then runs:

```
make lint
  └─ shell: /usr/local/bin/run.sh super-linter || record status
       shell: /usr/local/bin/run.sh go          || record status
       exit 1 if either step failed
```

Both Super-Linter and the custom Go linter are executed, and the final exit code reflects either failure. This keeps `lint_go_modules` invoked through the Makefile in CI while still running both linters.

## `run.sh` behavior

`/usr/local/bin/run.sh` is the container entry point. It accepts one optional argument:

| Argument | Behavior |
| --- | --- |
| `run.sh` (no argument) | Same as `run.sh all`. Runs Super-Linter for non-Go files, then the custom Go linter. |
| `run.sh super-linter` | Runs only Super-Linter. |
| `run.sh go` | Runs only `lint_go_modules`, the custom per-module Go linter. |
| `run.sh all` | Runs Super-Linter, then `lint_go_modules`, and exits non-zero if either fails. |

### Inside `run.sh`

1. **Prepare rules**
   - Copy `prechecks/base-linters/` from the image to `/tmp/platform-ci-toolkit/base-liners`.
   - If the mounted repo has `prechecks/custom-liners/`, copy those files on top.
   - Set `LINTER_RULES_PATH=/tmp/platform-ci-toolkit/base-liners`.

2. **Load environment**
   - Source `/etc/platform-ci-toolkit/super-linter.env` (baseline).
   - Source `/tmp/lint/prechecks/custom-liners/super-linter.env` if it exists (consumer overrides).

3. **Run the requested command**
   - `super-linter`: executes `/action/lib/linter.sh`.
   - `go`: executes `lint_go_modules()`:
     - Finds every `go.mod` under `/tmp/lint`, skipping `vendor/` and `.git/`.
     - Respects `FILTER_REGEX_INCLUDE` if set (used by `make lint-module`).
     - Without a filter, limits modules to those with changed `.go` files vs `DEFAULT_BRANCH`.
      - Runs `golangci-lint run --timeout=15m --config <merged> ./...` once per selected module from inside that module directory.
   - `all`: runs both and returns a non-zero exit code if either part fails.

## Custom Go linter

Super-Linter's `GO_MODULES` mode loads every `go.mod` in the repository at once. On multi-module repositories such as Cloud Backuper this exhausts the linter and times out. The custom Go linter instead runs `golangci-lint` once per module from inside the module directory:

```bash
cd services/backuper-service && golangci-lint run --config .golangci.yml ./...
cd services/trimmer-velero && golangci-lint run --config .golangci.yml ./...
```

By default, only modules that contain changed `.go` files (compared with `DEFAULT_BRANCH`) are linted. When `FILTER_REGEX_INCLUDE` is set, for example via `make lint-module MODULE=...`, only matching modules are linted.

## Configuration

Baseline rules live in `prechecks/base-linters` inside the linter image. A consumer repository can override them by placing files in `prechecks/custom-liners/`, including a `super-linter.env` file for environment overrides.

The baseline `super-linter.env` disables Super-Linter's own Go linters (`VALIDATE_GO` and `VALIDATE_GO_MODULES`) and enables `PLATFORM_CI_CUSTOM_GO_LINT=true`. The custom per-module Go linter is therefore responsible for all Go linting. Set `PLATFORM_CI_CUSTOM_GO_LINT=false` to fall back to Super-Linter's default Go behavior.

## Private Go module authentication

There is no shared Go module or build cache. The per-module `golangci-lint` runs rely on the normal Go download behavior inside the container. Earlier trials that used a `.go-cache` directory were removed to keep the integration simple and avoid accidentally linting cached dependency files.

If a module depends on a private repository (for example, `git.netcracker.com`), `go` needs credentials inside the container. Without them, the custom Go linter appears to hang while `go` waits for a username; the current `golangci-lint` timeout limits this to 15 minutes and the run fails with an authentication error. To lint such modules, choose one of the following approaches:

1. **Use an internal Go module proxy (recommended if available).** If your organization publishes private modules to an internal proxy such as Artifactory, set `GOPROXY` and optionally `GOSUMDB`/`GONOSUMDB` before running the linter. The consumer Makefile forwards these variables into the container automatically:
   ```bash
   export GOPROXY='https://artifactorycn.netcracker.com/pd.saas.golang.dev,https://artifactorycn.netcracker.com/pd.saas-release.golang.group'
   export GOSUMDB=off
   make lint-module MODULE=backuper-service
   ```

2. **Configure GitLab `CI_JOB_TOKEN` in CI (recommended for GitLab).** Add this step to the Super Linter job before `make lint`:
   ```bash
   git config --global url."https://gitlab-ci-token:${CI_JOB_TOKEN}@git.netcracker.com/".insteadOf "https://git.netcracker.com/"
   make lint
   ```
   `CI_JOB_TOKEN` is provided automatically by GitLab runners.

3. **Mount a pre-populated Go module cache.** Mount the host or CI cache `go/pkg/mod` directory into the container and set `GOMODCACHE=/go/pkg/mod`. The cache must already contain the exact versions of any private modules; otherwise `go` will still try to fetch them.

4. **Mount a `.netrc` file or inject a token.** Provide a `.netrc` with machine credentials, or pass a token as an environment variable and configure `git config url.<token-url>.insteadOf` before linting.
    - For GitLab personal/group access tokens, use `https://oauth2:<token>@git.netcracker.com/` as the rewritten URL.
    - For GitLab CI job tokens, use `https://gitlab-ci-token:${CI_JOB_TOKEN}@git.netcracker.com/`.

Without one of these, `make lint-module` on repositories such as Cloud Backuper fails while fetching `git.netcracker.com/prod.platform.cloud_core/libs/go/security/v3`.
