# Using the common linter

This document explains how to use the `platform-ci-toolkit` common linter locally, in CI pipelines, and in GitHub Actions workflows.

The common linter image is published as `ghcr.io/netcracker/linter` and is based on GitHub Super-Linter v9. Consumer repositories copy the `templates/Makefile.consumer` template so the same targets work locally and in CI.

## Default linters

The common linter does not enable linters explicitly. Instead, it relies on Super-Linter's auto-detection and only disables languages that are not commonly used in the target repositories.

- `super-linter.env` in this repository sets `VALIDATE_<LANGUAGE>=false` for languages such as Java, Kotlin, Terraform, XML, JavaScript, TypeScript, Ruby, PHP, etc.
- All other Super-Linter v9 languages are auto-detected: when a changed file matches a language, its linters run automatically.

For the full list of supported linters and the meaning of each `VALIDATE_*` variable, see the upstream Super-Linter documentation:

- [Supported linters](https://github.com/super-linter/super-linter#supported-linters)
- [Environment variables](https://github.com/super-linter/super-linter/blob/main/docs/run-linter-locally.md)

A consumer repository can override the baseline by placing `prechecks/custom-liners/super-linter.env` in its own repository.

---

## Local usage

### Prerequisites

- Docker installed and running.
- A consumer `Makefile` based on `templates/Makefile.consumer` copied into the consumer repository.
- The repository root mounted at `/tmp/lint` inside the container (handled by the Makefile).

### Makefile targets

| Target | Purpose |
| --- | --- |
| `make lint` | Run Super-Linter on changed files only. Set `FILTER=<regex>` to lint only matching files. |
| `make lint-fix` | Run Super-Linter with auto-fix enabled for supported languages. |
| `make verify` | Run lint followed by tests. |

### Consumer Makefile setup

Copy `templates/Makefile.consumer` from `platform-ci-toolkit/linter` into the consumer repository as `Makefile`. Adjust the variables at the top of the file if needed.

### Run on changed files

```bash
make lint
```

Super-Linter compares `HEAD` against `DEFAULT_BRANCH` (default `master`) and lints only the changed files. Set `VALIDATE_ALL_CODEBASE=false` in the environment or in `super-linter.env`.

### Run on a subset of files

For repositories with multiple Go modules, linting the whole repo can be slow or time out. Set `FILTER` to lint a single module or directory:

```bash
make lint FILTER=services/backuper-service/.*
```

This still runs the **full** Super-Linter suite (Markdown, YAML, Dockerfile, Go, Python, etc.), but only against files matching the regex.

### Run with auto-fix

```bash
make lint-fix
```

This runs Super-Linter with the following fix-mode variables enabled:

| Variable | What it fixes |
| --- | --- |
| `FIX_GO=true` | Go formatting (`gofmt` / `goimports`). |
| `FIX_MARKDOWN=true` | Markdown formatting. |
| `FIX_YAML_PRETTIER=true` | YAML formatting via Prettier. |

After the run, review the changes with `git diff` and commit them.

#### `lint-fix` assumptions and caveats

- **Only changed files are fixed.** Because `VALIDATE_ALL_CODEBASE=false` is the default, fixes apply only to files that differ from `DEFAULT_BRANCH` (`master`). Uncommitted working-tree changes are not included unless they are part of commits ahead of `master`.
- **Not all linters support auto-fix.** The consumer template only enables fix mode for Go, Markdown, and YAML. Linters such as `GO_MODULES` (`errcheck`, `staticcheck`, etc.), `PYLINT`, `MYPY`, and `RUFF` report issues that must be fixed manually.
- **Fixes are written to the working tree.** The consumer Makefile mounts the repository root into the container, so Super-Linter can modify files directly. Run `git diff` afterwards to review and commit only the intended changes.
- **Python auto-fix is not enabled by default.** If you want Ruff or Black to auto-fix Python files, extend the `lint-fix` target with `FIX_PYTHON_RUFF=true` or `FIX_PYTHON_BLACK=true` (supported by Super-Linter v9).

### Run tests and verify

The consumer Makefile template includes test targets, but tests run **outside** the common linter container, either locally on a host with the required toolchain or in a dedicated CI job.

```bash
make verify      # Run lint, tests, and skillcheck
make test        # Run test-go and test-python
make test-go     # Auto-detect Go modules and run go test ./... where tests exist
make test-python # Placeholder; override in the consumer Makefile
make skillcheck  # Validate skill packages with the skillcheck image
```

`make test-go` auto-detects Go modules by finding `go.mod` files (excluding `vendor/`), checks whether each module contains any `*_test.go` files, and runs `go test ./...` only for those modules. By default, Go tests run only if `.go` or `go.mod` files changed against `DEFAULT_BRANCH` (`master`). If no relevant files changed, the target prints a skip message and exits successfully.

If `FILTER` is set, only modules whose path matches the regex are tested. For example:

```bash
make verify FILTER=services/backuper-service/.*
```

This runs lint and tests only for `services/backuper-service` and its subpackages.

To target multiple folders, use a regex alternation:

```bash
make verify FILTER='services/(backuper-service|trimmer-velero)/.*'
```

`FILTER` is passed directly to Super-Linter as `FILTER_REGEX_INCLUDE` and is also used by `test-go` to select Go modules. Quote the value so the shell preserves the regex.

`GO_TEST_MODULES` can be set explicitly to override auto-detection:

```make
GO_TEST_MODULES := \
	services/backuper-service \
	services/trimmer-velero
```

`make test-python` also checks for changed `.py` files against `DEFAULT_BRANCH` before running. The actual test command is controlled by `PYTHON_TEST_CMD` and must be configured by the consumer. If no Python files changed, the target skips.

`make skillcheck` validates skill packages using the `skillcheck` container image. Set `SKILLCHECK_PATTERNS` to one or more space-separated globs relative to the repository root:

```make
SKILLCHECK_PATTERNS := agent-packages/*/.apm/skills
```

The default pattern is `agent-packages/*/.apm/skills`. `make verify` includes `skillcheck`. If `SKILLCHECK_PATTERNS` is empty or no directories match, `skillcheck` prints a skip message and exits successfully.

Do not run tests inside the common linter container — it is built for static analysis only and may lack the Go toolchain, Python dependencies, or runtime services needed for tests.

---

## CI usage

### GitLab CI

The common linter image is designed to run inside a CI job. The CI runner disables the image entry point and executes `make lint` as the container command.

Key points:

- The container image is `ghcr.io/netcracker/linter:latest`.
- The entry point must be disabled with `entrypoint: [""]` (GitLab CI) or `--entrypoint=""` (other runners).
- `CI=true` is set automatically by GitLab runners; the consumer Makefile uses it to select the CI branch.
- `RUN_LOCAL=true` is required because Super-Linter is not running inside GitHub Actions.

Example `.gitlab-ci.yml` job:

```yaml
super-linter:
  stage: lint
  image:
    name: ghcr.io/netcracker/linter:latest
    entrypoint: [""]
  variables:
    RUN_LOCAL: "true"
    DEFAULT_BRANCH: "master"
    VALIDATE_ALL_CODEBASE: "false"
  script:
    - make lint
```

For private Go modules hosted on an internal Git server, configure GitLab authentication before linting:

```yaml
  script:
    - git config --global url."https://gitlab-ci-token:${CI_JOB_TOKEN}@git.<private-git-host>.com/".insteadOf "https://git.<private-git-host>.com/"
    - make lint
```

### GitHub Actions

Use the published image in a workflow step and disable the entry point:

```yaml
jobs:
  lint:
    runs-on: ubuntu-latest
    container:
      image: ghcr.io/netcracker/linter:latest
      options: --entrypoint=""
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - name: Run common linter
        run: make lint
        env:
          RUN_LOCAL: "true"
          DEFAULT_BRANCH: "master"
          VALIDATE_ALL_CODEBASE: "false"
```

For private Go modules, pass a token or configure `git config url.<token-url>.insteadOf` before `make lint`.

### Reusable workflows

The consumer `Makefile` abstracts the local-vs-CI difference. In CI the job runs inside the linter container, so the Makefile target is simply:

```bash
/usr/local/bin/run.sh super-linter
```

Locally, the Makefile pulls the image and runs the equivalent Docker command.

---

## Environment variables

The consumer Makefile forwards the following variables into the linter container:

| Variable | Purpose |
| --- | --- |
| `RUN_LOCAL` | Must be `true` when not running inside GitHub Actions. |
| `DEFAULT_BRANCH` | Branch Super-Linter compares against when `VALIDATE_ALL_CODEBASE=false`. |
| `VALIDATE_ALL_CODEBASE` | `false` lints changed files only; `true` lints everything. |
| `FILTER_REGEX_INCLUDE` | Regex for files to include (set via `FILTER`). |
| `FILTER_REGEX_EXCLUDE` | Regex for files to exclude. |
| `GOPROXY` | Go module proxy for private modules. |

Additional Super-Linter variables can be set via `prechecks/custom-liners/super-linter.env` in the consumer repository.

> **Go modules and private repositories:** `run.sh` prints a warning at the start of the lint run if it finds `go.mod` files and `GOPROXY` is not set. This is a heads-up that `GO_MODULES` may fail when downloading private dependencies. Set `GOPROXY` or configure Git authentication to avoid the failure.

---

## Troubleshooting

### `make lint` times out on multi-module repositories

Set `FILTER` to lint one module at a time:

```bash
make lint FILTER=services/backuper-service/.*
```

### Private Go modules fail to download

`run.sh` prints an early warning when `go.mod` files are present and `GOPROXY` is not set. If you see that warning and `GO_MODULES` later fails with an authentication error, configure one of these authentication methods:

- Internal `GOPROXY` (set in the environment).
- GitLab `CI_JOB_TOKEN` with `git config url.<token-url>.insteadOf`.
- Pre-populated `GOMODCACHE` mount.
- `.netrc` or personal access token.

### `lint-fix` does not fix Python or Go module issues

`lint-fix` only auto-fixes formatting. Code-quality issues reported by `GO_MODULES`, `PYLINT`, `MYPY`, or `RUFF` must be fixed manually. Add `FIX_PYTHON_RUFF=true` or `FIX_PYTHON_BLACK=true` to the target if you want Python auto-fix.

---
