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
- A consumer `Makefile` based on `templates/Makefile.consumer`.
- The repository root mounted at `/tmp/lint` inside the container (handled by the Makefile).

### Makefile targets

| Target | Purpose |
| --- | --- |
| `make lint` | Run Super-Linter on changed files only. |
| `make lint-filter FILTER=<regex>` | Run Super-Linter on files matching a regex relative to the repo root. |
| `make lint-fix` | Run Super-Linter with auto-fix enabled for supported languages. |

### Run on changed files

```bash
make lint
```

Super-Linter compares `HEAD` against `DEFAULT_BRANCH` (default `master`) and lints only the changed files. Set `VALIDATE_ALL_CODEBASE=false` in the environment or in `super-linter.env`.

### Run on a subset of files

For repositories with multiple Go modules, linting the whole repo can be slow or time out. Use `lint-filter` to lint a single module or directory:

```bash
make lint-filter FILTER=services/backuper-service/.*
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

For private `git.netcracker.com` Go modules, configure GitLab authentication before linting:

```yaml
  script:
    - git config --global url."https://gitlab-ci-token:${CI_JOB_TOKEN}@git.netcracker.com/".insteadOf "https://git.netcracker.com/"
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
| `FILTER_REGEX_INCLUDE` | Regex for files to include (used by `lint-filter`). |
| `FILTER_REGEX_EXCLUDE` | Regex for files to exclude. |
| `GOPROXY` / `GOSUMDB` | Go module proxy and sum-database settings for private modules. |

Additional Super-Linter variables can be set via `prechecks/custom-liners/super-linter.env` in the consumer repository.

---

## Troubleshooting

### `make lint` times out on multi-module repositories

Use `make lint-filter` to lint one module at a time:

```bash
make lint-filter FILTER=services/backuper-service/.*
```

### Private Go modules fail to download

Ensure one of the authentication methods from `docs/linting.md` is configured:

- Internal `GOPROXY` (set in the environment).
- GitLab `CI_JOB_TOKEN` with `git config url.<token-url>.insteadOf`.
- Pre-populated `GOMODCACHE` mount.
- `.netrc` or personal access token.

### `lint-fix` does not fix Python or Go module issues

`lint-fix` only auto-fixes formatting. Code-quality issues reported by `GO_MODULES`, `PYLINT`, `MYPY`, or `RUFF` must be fixed manually. Add `FIX_PYTHON_RUFF=true` or `FIX_PYTHON_BLACK=true` to the target if you want Python auto-fix.

---
