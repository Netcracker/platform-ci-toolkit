# Go linting with the common linter

This document describes the Go linting behavior of the platform-ci-toolkit common linter. The image is based on GitHub Super-Linter v9 and uses Super-Linter’s own `GO` and `GO_MODULES` checks; there is no separate custom Go linter.

## How it works

The wrapper script `/usr/local/bin/run.sh` exposes two commands:

| Command | What it runs |
| --- | --- |
| `run.sh` (no argument) | Same as `run.sh all`. Runs Super-Linter. |
| `run.sh super-linter` | Runs Super-Linter. |
| `run.sh all` | Runs Super-Linter. |

Consumer repositories invoke these commands through the `Makefile` template so the same targets work both in CI pipelines and locally.

## Makefile targets

```bash
make lint                              # Super-Linter on changed files
make lint-filter FILTER=services/backuper-service/.*  # Super-Linter on a filtered set of files
make lint-fix                          # Super-Linter with auto-fix enabled
```

`make lint-filter` runs the **full** Super-Linter suite (Markdown, YAML, JSON, Dockerfile, Go, etc.) but only against files matching the `FILTER` regex.

In CI, the reusable workflow runs inside the linter container with `--entrypoint=""`, `RUN_LOCAL=true`, and executes `make lint`. `RUN_LOCAL=true` is required whenever Super-Linter is not running inside GitHub Actions, so CI pipelines set it as well.

> **Note:** `CI=true` is not set manually in any file — GitLab CI and GitHub Actions runners set it automatically. The Makefiles simply check `ifeq ($(CI),true)` to choose the CI branch.

## What happens when you run `make lint`

### Local mode (`CI` is unset)

```
make lint
  └─ ensure-linter-image
       └─ docker run ghcr.io/netcracker/linter:latest super-linter
            └─ ENTRYPOINT /usr/local/bin/run.sh super-linter
                 1. Copy base rules from /etc/platform-ci-toolkit/base-linters
                 2. Overlay /tmp/lint/prechecks/custom-liners/*
                 3. Load /etc/platform-ci-toolkit/super-linter.env
                 4. Load /tmp/lint/prechecks/custom-liners/super-linter.env
                 5. /action/lib/linter.sh   (Super-Linter)
```

The repository root is mounted at `/tmp/lint`. `run.sh` writes the merged rule set to a temporary directory outside the mount so the working tree is never modified.

### CI mode (`CI=true`)

The CI runner starts a container from the linter image with the entry point disabled, then runs:

```
make lint
  └─ /usr/local/bin/run.sh super-linter
```

## `run.sh` behavior

`/usr/local/bin/run.sh` is the container entry point. It accepts one optional argument:

| Argument | Behavior |
| --- | --- |
| `run.sh` (no argument) | Runs Super-Linter. |
| `run.sh super-linter` | Runs Super-Linter. |
| `run.sh all` | Runs Super-Linter. |

### Inside `run.sh`

1. **Prepare rules**
   - Copy `prechecks/base-linters/` from the image to `/tmp/platform-ci-toolkit/base-linters`.
   - If the mounted repo has `prechecks/custom-liners/`, copy those files on top.
   - Set `LINTER_RULES_PATH=/tmp/platform-ci-toolkit/base-linters`.

2. **Load environment**
   - Source `/etc/platform-ci-toolkit/super-linter.env` (baseline).
   - Source `/tmp/lint/prechecks/custom-liners/super-linter.env` if it exists (consumer overrides).

3. **Run Super-Linter**
   - Execute `/action/lib/linter.sh`.

## Configuration

Baseline rules live in `prechecks/base-linters` inside the linter image. A consumer repository can override them by placing files in `prechecks/custom-linters/`, including a `super-linter.env` file for environment overrides.

The baseline `super-linter.env` disables languages that are not commonly used; Go is **not** disabled, so Super-Linter auto-detects and runs `GO` / `GO_MODULES`.

## Private Go module authentication

There is no shared Go module or build cache. Super-Linter’s `GO_MODULES` relies on the normal Go download behavior inside the container.

If a module depends on a private repository (for example, `git.netcracker.com`), `go` needs credentials inside the container. Without them, the linter may hang or fail with an authentication error while fetching dependencies. To lint such modules, choose one of the following approaches:

1. **Use an internal Go module proxy (recommended if available).** If your organization publishes private modules to an internal proxy such as Artifactory, set `GOPROXY` and optionally `GOSUMDB`/`GONOSUMDB` before running the linter. The consumer Makefile forwards these variables into the container automatically:
   ```bash
   export GOPROXY='https://artifactorycn.netcracker.com/pd.saas.golang.dev,https://artifactorycn.netcracker.com/pd.saas-release.golang.group'
   export GOSUMDB=off
   make lint
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

Without one of these, `make lint` on repositories such as Cloud Backuper fails while fetching `git.netcracker.com/prod.platform.cloud_core/libs/go/security/v3`.
