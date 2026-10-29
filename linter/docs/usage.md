# Using the common linter

This document explains how to use the `platform-ci-toolkit` common linter locally, in CI pipelines, and in GitHub Actions workflows.

The common linter image is published as `ghcr.io/netcracker/linter` and is based on GitHub Super-Linter v9. It also includes the `skillcheck` utility for validating Agent Skills. Consumer repositories copy the `templates/Makefile.consumer` template so the same targets work locally and in CI.

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
| `make lint` | Run Super-Linter and skill validation (when `SKILLCHECK_PATTERNS` is set). Set `FILTER_REGEX_INCLUDE=<regex>` to lint only matching files. |
| `make lint-super-linter` | Run Super-Linter only. |
| `make skill-validate` | Run `skillcheck` only (skips if `SKILLCHECK_PATTERNS` is empty). |
| `make lint-fix` | Run Super-Linter with auto-fix enabled for supported languages. |
| `make verify` | Run lint followed by tests. This is the target CI should call when a single job is preferred. |
| `make test-java` | Run Maven tests in every detected module when Java/XML files changed against `DEFAULT_BRANCH`. Skips when no `pom.xml` is found. |

### Consumer Makefile setup

Copy `templates/Makefile.consumer` from `platform-ci-toolkit/linter` into the consumer repository as `Makefile`. Adjust the variables at the top of the file if needed.

### Run on changed files

```bash
make lint
```

Super-Linter compares `HEAD` against `DEFAULT_BRANCH` (default `master`) and lints only the changed files. Set `VALIDATE_ALL_CODEBASE=false` in the environment or in `super-linter.env`.

### Run on a subset of files

For repositories with multiple Go modules, linting the whole repo can be slow or time out. Set `FILTER_REGEX_INCLUDE` to lint a single module or directory:

```bash
make lint FILTER_REGEX_INCLUDE=services/backuper-service/.*
```

This still runs the **full** Super-Linter suite (Markdown, YAML, Dockerfile, Go, Python, etc.), but only against files matching the regex.

### Skill validation

`make lint` also runs `make skill-validate` when `SKILLCHECK_PATTERNS` is set in the consumer `Makefile` or environment. `make skill-validate` invokes `skillcheck` directly and can be run on its own.

Patterns are shell globs relative to the repository root that point to skill bundles or individual skill folders. Directories that do not contain a `SKILL.md` file (or a direct subdirectory with one) are silently ignored, so generic patterns do not fail the lint job when no skills are present.

Example in a consumer `Makefile`:

```make
SKILLCHECK_PATTERNS ?= .opencode/skills
```

In CI, set the same variable in the job variables:

```yaml
  variables:
    SKILLCHECK_PATTERNS: ".opencode/skills"
```

If `SKILLCHECK_PATTERNS` is empty, `make skill-validate` prints a skip message and exits successfully, so `make lint` still runs Super-Linter only.

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

The consumer Makefile template includes test targets. Locally, `make verify` runs the linter inside the common linter container and runs tests on the host. In CI, lint and tests each run inside the common linter container.

```bash
make verify      # Run lint and tests
make test        # Run test-go, test-python, and test-java
make test-go     # Auto-detect Go modules and run go test ./... where tests exist
make test-python # Placeholder; override in the consumer Makefile
make test-java   # Auto-detect Maven modules and run mvn test when Java/XML changed
```

`make test-go` auto-detects Go modules by finding `go.mod` files (excluding `vendor/`), checks whether each module contains any `*_test.go` files, and runs `go test ./...` for every module that has tests. By default, `test-go` compares the working tree against `DEFAULT_BRANCH` (`master`) using `git diff --name-only DEFAULT_BRANCH --`, so it includes both committed branch changes and uncommitted working-tree changes. If no `.go`, `go.mod`, or `.sum` files changed, the target prints a skip message and exits successfully. Otherwise it runs tests in every detected module.

Set `FILTER_REGEX_INCLUDE` directly when you need to narrow the lint scope. It does not affect `test-go` or `test-python`; if any matching-language files changed, all tests for that language run. Quote the value so the shell preserves the regex.

To lint a single module:

```bash
make lint FILTER_REGEX_INCLUDE=services/backuper-service/.*
```

To target multiple folders with lint, use a regex alternation:

```bash
make lint FILTER_REGEX_INCLUDE='services/(backuper-service|trimmer-velero)/.*'
```

`GO_TEST_MODULES` can be set explicitly to override auto-detection:

```make
GO_TEST_MODULES := \
	services/backuper-service \
	services/trimmer-velero
```

`make test-python` checks for changed `.py` files against `DEFAULT_BRANCH` before running. If no Python files changed, the target skips.

`PY_BUILD` must be set for Python tests to run. This is intentional: the consumer must explicitly define how Python dependencies are installed. If `PY_BUILD` is empty, `test-python` prints a skip message and exits successfully.

The Python interpreter defaults to `python3`. Override `PYTHON` if the host or CI container only provides `python`:

```make
PYTHON := python
```

When `PY_BUILD` is set, `test-python` performs these steps:

1. **Build step:** `PY_BUILD` is executed first. Use this to install Python requirements or to build anything the tests need. Examples:

   ```make
   PY_BUILD := $(PYTHON) -m pip install -r requirements.txt
   PY_BUILD := make py_build
   ```

2. **Install pytest:** `$(PYTHON) -m pip install --quiet pytest` runs to ensure the test runner is available.

3. **Run tests:** By default `$(PYTHON) -m pytest` is executed. Override `PYTHON_TEST_CMD` to use a different runner or arguments:

   ```make
   PYTHON_TEST_CMD := $(PYTHON) -m pytest -q
   ```

`make test-java` auto-detects Maven modules by finding `pom.xml` files (excluding `*/target/*`), checks whether any `.java` or `.xml` files changed against `DEFAULT_BRANCH`, and runs `mvn test` (override with `JAVA_BUILD`) in every detected module. If no `pom.xml` is found or no Java/XML files changed, the target prints a skip message and exits successfully. The common linter image includes Maven and OpenJDK 25, so `make test-java` works out of the box in CI. If a project needs a different JDK version, override the test job to use a Java-enabled image or set `JAVA_BUILD` accordingly.

CI pipelines can run lint and tests as two parallel jobs (shown below) or call `make verify` so that lint and tests share a single entry point and a single pass/fail result. The common linter image is based on Super-Linter and includes the Go, Python and Java runtimes used by `go test`, the Python test command, and `mvn test`.

---

## CI usage

### GitLab CI

The common linter image is designed to run inside CI jobs. The CI runner disables the image entry point and executes `make lint` or `make test` (or `make verify`) as the container command.

Key points:

- The container image is `ghcr.io/netcracker/linter:latest`.
- The entry point must be disabled with `entrypoint: [""]` (GitLab CI) or `--entrypoint=""` (other runners).
- `CI=true` is set automatically by GitLab runners; the consumer Makefile uses it to select the CI branch.
- `RUN_LOCAL=true` is required because Super-Linter is not running inside GitHub Actions.
- The common linter expects the repository at `/tmp/lint`. `run.sh` automatically links `/tmp/lint` to `CI_PROJECT_DIR` (GitLab) or `GITHUB_WORKSPACE` (GitHub Actions) when the expected path is missing.
- The image sets `GOMODCACHE=/tmp/lint/.go/pkg/mod` so Go modules are cached inside the repository and can be saved/restored by CI. `GOPROXY` is not hardcoded in the image; set it in your CI variables if the default public proxy is unreachable or you need an internal proxy.

Example `.gitlab-ci.yml` jobs:

```yaml
lint:
  stage: lint
  image:
    name: ghcr.io/netcracker/linter:latest
    entrypoint: [""]
  variables:
    RUN_LOCAL: "true"
    DEFAULT_BRANCH: "master"
    VALIDATE_ALL_CODEBASE: "false"
    # Set GOPROXY as a CI/CD or group variable if the default public proxy
    # is unreachable or you need a private module proxy.
  cache:
    - key: "go-mod-cache-$CI_PROJECT_NAME-lint"
      paths:
        - .go/pkg/mod
  script:
    # Allow the Go toolchain to fetch private git.netcracker.com modules.
    # CI_JOB_TOKEN is provided automatically by GitLab runners.
    - git config --global url."https://gitlab-ci-token:${CI_JOB_TOKEN}@git.netcracker.com/".insteadOf "https://git.netcracker.com/"
    - make lint

tests:
  stage: lint
  image:
    name: ghcr.io/netcracker/linter:latest
    entrypoint: [""]
  variables:
    DEFAULT_BRANCH: "master"
    # Set GOPROXY as a CI/CD or group variable if the default public proxy
    # is unreachable or you need a private module proxy.
    # If Python tests need a private PyPI proxy, place a pip.conf in the repo
    # root and point PIP_CONFIG_FILE at it.
    PIP_CONFIG_FILE: "$CI_PROJECT_DIR/pip.conf"
    PIP_CACHE_DIR: "$CI_PROJECT_DIR/.pip-cache"
    MAVEN_OPTS: "-Dmaven.repo.local=$CI_PROJECT_DIR/.m2/repository"
  cache:
    - key: "go-mod-cache-$CI_PROJECT_NAME"
      paths:
        - .go/pkg/mod
    - key: "pip-cache-$CI_PROJECT_NAME"
      paths:
        - .pip-cache
    - key: "maven-cache-$CI_PROJECT_NAME"
      paths:
        - .m2/repository
  script:
    # Allow the Go toolchain to fetch private git.netcracker.com modules.
    # CI_JOB_TOKEN is provided automatically by GitLab runners.
    - git config --global url."https://gitlab-ci-token:${CI_JOB_TOKEN}@git.netcracker.com/".insteadOf "https://git.netcracker.com/"
    - make test

java-tests:
  stage: lint
  image:
    name: maven:3.9-eclipse-temurin-17
    entrypoint: [""]
  variables:
    DEFAULT_BRANCH: "master"
  rules:
    - changes:
        - "**/*.java"
        - "**/pom.xml"
  script:
    - git fetch origin ${DEFAULT_BRANCH} --depth=1
    - make test-java
```

If Python tests download packages from a private PyPI index, place a `pip.conf` at the repository root and set `PIP_CONFIG_FILE: "$CI_PROJECT_DIR/pip.conf"` in the test job. The path can be changed by the consumer if needed.

For private Go modules hosted on an internal Git server, configure GitLab authentication before linting:

```yaml
  script:
    - git config --global url."https://gitlab-ci-token:${CI_JOB_TOKEN}@git.<private-git-host>.com/".insteadOf "https://git.<private-git-host>.com/"
    - make verify
```

### GitHub Actions

Use the published image in a workflow and disable the entry point. Two jobs in the same workflow run lint and tests in parallel; add a Java-specific job if the repository contains Maven modules:

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
      - name: Configure Git authentication for private Go modules
        run: |
          git config --global url."https://token:${{ secrets.GITHUB_TOKEN }}@github.com/".insteadOf "https://github.com/"
      - name: Run lint
        run: make lint
        env:
          RUN_LOCAL: "true"
          DEFAULT_BRANCH: "master"
          VALIDATE_ALL_CODEBASE: "false"

  tests:
    runs-on: ubuntu-latest
    container:
      image: ghcr.io/netcracker/linter:latest
      options: --entrypoint=""
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - name: Cache Go modules
        uses: actions/cache@v4
        with:
          path: .go/pkg/mod
          key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
          restore-keys: |
            ${{ runner.os }}-go-
      - name: Cache Python packages
        uses: actions/cache@v4
        with:
          path: .pip-cache
          key: ${{ runner.os }}-pip-${{ hashFiles('**/requirements*.txt') }}
          restore-keys: |
            ${{ runner.os }}-pip-
      - name: Cache Maven packages
        uses: actions/cache@v4
        with:
          path: .m2/repository
          key: ${{ runner.os }}-maven-${{ hashFiles('**/pom.xml') }}
          restore-keys: |
            ${{ runner.os }}-maven-
      - name: Configure Git authentication for private Go modules
        run: |
          git config --global url."https://token:${{ secrets.GITHUB_TOKEN }}@github.com/".insteadOf "https://github.com/"
      - name: Run tests
        run: make test
        env:
          DEFAULT_BRANCH: "master"
          # Set GOPROXY if the default public proxy is unreachable or you need a private module proxy.
          GOPROXY: ${{ vars.GOPROXY }}
          PIP_CACHE_DIR: "${{ github.workspace }}/.pip-cache"
          MAVEN_OPTS: "-Dmaven.repo.local=${{ github.workspace }}/.m2/repository"

  java-tests:
    runs-on: ubuntu-latest
    if: hashFiles('**/pom.xml') != ''
    container:
      image: maven:3.9-eclipse-temurin-17
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - name: Run Java tests
        run: make test-java
        env:
          DEFAULT_BRANCH: "master"
```

For private Go modules, adjust the `git config url.<token-url>.insteadOf` line to match your git host and token.

### Reusable workflows

The consumer `Makefile` abstracts the local-vs-CI difference. In CI the job should run `make lint` and `make test` inside the linter container; both use the same image and the same `DEFAULT_BRANCH` logic. Locally, `make lint` pulls the linter image and runs Super-Linter inside it, while `make test` runs Go/Python tests directly on the host.

---

## Environment variables

The consumer Makefile forwards the following variables into the linter container:

| Variable | Purpose |
| --- | --- |
| `RUN_LOCAL` | Must be `true` when not running inside GitHub Actions. |
| `DEFAULT_BRANCH` | Branch Super-Linter compares against when `VALIDATE_ALL_CODEBASE=false`. |
| `VALIDATE_ALL_CODEBASE` | `false` lints changed files only; `true` lints everything. |
| `FILTER_REGEX_INCLUDE` | Regex for files to include. Set directly, e.g. `make lint FILTER_REGEX_INCLUDE=...`. |
| `FILTER_REGEX_EXCLUDE` | Regex for files to exclude. |
| `GOPROXY` | Go module proxy. Set this in CI when the default public proxy is unreachable or for private modules. |
| `GOMODCACHE` | Location of the Go module cache. The image defaults to `/tmp/lint/.go/pkg/mod` so CI systems can cache it inside the repository. |
| `PIP_CACHE_DIR` | Location of the pip download/wheel cache. Set this to a project-local path (e.g. `$CI_PROJECT_DIR/.pip-cache` or `${{ github.workspace }}/.pip-cache`) so CI can save/restore it. |
| `MAVEN_OPTS` | Maven options, e.g. `-Dmaven.repo.local=<project-local-path>/.m2/repository` so CI can cache the Maven local repository inside the repository. |
| `SKILLCHECK_PATTERNS` | Shell globs for skill bundles or individual skills to validate with `skillcheck`. Empty (default) disables skill validation. |

Additional Super-Linter variables can be set via `prechecks/custom-liners/super-linter.env` in the consumer repository.

For Python tests, `make test` reads `PIP_CONFIG_FILE` when it runs `pip install`. If your tests need a private PyPI proxy, place a `pip.conf` at the repository root and set `PIP_CONFIG_FILE: "$CI_PROJECT_DIR/pip.conf"` in the CI test job. This is not forwarded into the linter container because it is only used by the test target.

> **Go modules and private repositories:** `run.sh` prints a warning at the start of the lint run if it finds `go.mod` files and `GOPROXY` is not set. This is a heads-up that `GO_MODULES` may fail when downloading private dependencies. Set `GOPROXY` or configure Git authentication to avoid the failure.

---

## Troubleshooting

### `The workspace (/tmp/lint) is not a directory!`

Super-Linter expects the repository at `/tmp/lint`. The consumer Makefile mounts the repo there for local runs; in CI, `run.sh` links `/tmp/lint` to `CI_PROJECT_DIR` (GitLab) or `GITHUB_WORKSPACE` (GitHub Actions) automatically. If you see this error in CI, make sure the job uses a current common-linter image that includes the auto-link behavior.

### `make lint` times out on multi-module repositories

Set `FILTER_REGEX_INCLUDE` to lint one module at a time:

```bash
make lint FILTER_REGEX_INCLUDE=services/backuper-service/.*
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
