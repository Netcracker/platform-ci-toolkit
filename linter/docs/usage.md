# Using the common linter

This document explains how to use the `platform-ci-toolkit` common linter locally, in CI pipelines, and in GitHub Actions workflows.

The common linter image is published as `ghcr.io/netcracker/linter` and is based on GitHub Super-Linter v9. It also includes the `skillcheck` utility for validating Agent Skills. Consumer repositories either copy `templates/Makefile` as their own `Makefile`, or include it from a thin wrapper so the same targets work locally and in CI.

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
- A consumer `Makefile` based on `templates/Makefile` copied into the consumer repository, or a wrapper that includes the template.
- The repository root mounted at `/tmp/lint` inside the container (handled by the Makefile).

### Makefile targets

| Target | Purpose |
| --- | --- |
| `make lint` | Run Super-Linter and skill validation (when `SKILLCHECK_PATTERNS` is set). Set `FILTER_REGEX_INCLUDE=<regex>` to lint only matching files. |
| `make lint-super-linter` | Run Super-Linter only. |
| `make skill-validate` | Run `skillcheck` only (skips if `SKILLCHECK_PATTERNS` is empty). |
| `make lint-fix` | Run Super-Linter with auto-fix enabled for supported languages. |
| `make verify` | Run lint followed by tests. This is the target CI should call when a single job is preferred. |
| `make test` | Run language-specific tests together: `test-go`, `test-python`, and `test-java`. |
| `make test-go` | Auto-detect Go modules and run `go test ./...` where tests exist. Skips when no Go files changed. |
| `make test-python` | Run Python tests when `PY_BUILD` is set and Python files changed. Skips otherwise. |
| `make test-java` | Auto-detect Maven modules and run `mvn test` when Java/XML files changed against `DEFAULT_BRANCH`. Skips when no `pom.xml` is found or no files changed. |

### Consumer Makefile setup

There are two ways to consume the template. Prefer the include pattern so the shared recipe stays in one file you can refresh from upstream.

#### Include a vendored copy (recommended)

1. Copy `templates/Makefile` into the consumer repository as `Makefile.linter`.
2. Keep a thin consumer `Makefile` that sets only the overrides this repository needs, then includes `Makefile.linter`.

Example wrapper:

```make
# Consumer Makefile (overrides only)
REPO_ROOT := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

LINTER_IMAGE ?= ghcr.io/netcracker/linter
LINTER_TAG ?= common_linter_integration
DEFAULT_BRANCH ?= main

# Override only when this repo needs extra skill paths or a different layout.
# SKILLCHECK_PATTERNS ?= agent-packages/*/.apm/skills dbaas-operator/agent-packages/*/.apm/skills
# export SKILLCHECK_PATTERNS

PYTHON ?= python3
PY_BUILD ?= PYTHONPATH=tests/robot/lib $(PYTHON) -m pip install -r tests/robot/requirements.txt

# Evaluate against this checkout before Makefile.linter can overwrite REPO_ROOT.
GO_TEST_MODULES ?= $(shell find $(REPO_ROOT) -name go.mod -not -path '*/vendor/*' -exec dirname {} \;)
JAVA_TEST_MODULES ?= $(shell find $(REPO_ROOT) -name pom.xml -not -path '*/target/*' -exec dirname {} \;)

include $(REPO_ROOT)Makefile.linter

# Point REPO_ROOT back at this repository when Makefile.linter used lastword.
REPO_ROOT := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))
```

Refresh the vendored file when the upstream template changes:

```bash
make sync-linter-makefile PLATFORM_CI_TOOLKIT_RAW_URL=<raw-url-to-templates/Makefile>
```

`sync-linter-makefile` writes `Makefile.linter` at the repository root.

#### Copy the template as the main Makefile

Copy `templates/Makefile` into the consumer repository as `Makefile` and adjust the variables at the top. Use this when you do not want a separate `Makefile.linter`.

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

`make lint` also runs `make skill-validate` when `SKILLCHECK_PATTERNS` is non-empty. `make skill-validate` invokes `skillcheck` directly and can be run on its own.

The shared template defaults to:

```make
SKILLCHECK_PATTERNS ?= agent-packages/*/.apm/skills
```

Patterns are shell globs relative to the repository root. They point to skill bundles or individual skill folders. Directories that do not contain a `SKILL.md` file (or a direct subdirectory with one) are silently ignored, so a pattern that matches no skills does not fail the lint job.

Override in the consumer `Makefile` only when this repository needs a different path list. Do not set `SKILLCHECK_PATTERNS` again in the GitLab job variables or GitHub Actions `env` block. A CI environment value wins over Makefile `?=`, so a workflow override can drop paths that the Makefile already defines.

Example override in the consumer `Makefile` (not in the workflow):

```make
SKILLCHECK_PATTERNS ?= agent-packages/*/.apm/skills dbaas-operator/agent-packages/*/.apm/skills
export SKILLCHECK_PATTERNS
```

To skip skill validation, set an empty value in the consumer `Makefile`:

```make
SKILLCHECK_PATTERNS ?=
export SKILLCHECK_PATTERNS
```

Then `make skill-validate` prints a skip message and exits successfully, so `make lint` still runs Super-Linter only.

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

Define one shared `.pre-check-base` job and reuse it by setting `TARGET` to `lint` or `test` (the consumer Makefile targets). Do not duplicate the image, cache, and script blocks for each job.

Shared base (keep this in a template file such as `templates/pre-check-common.yml`, or paste it into `.gitlab-ci.yml`):

```yaml
.pre-check-base:
  stage: linter
  image:
    name: "artifactorycn.netcracker.com:17152/netcracker/linter:common_linter_integration"
    # The image entrypoint is "/usr/local/bin/run.sh"; GitLab needs a shell instead for tests.
    entrypoint: [""]
  variables:
    DEFAULT_BRANCH: "master"
    TARGET: ""
    PIP_CONFIG_FILE: "$CI_PROJECT_DIR/pip.conf"
    GOPROXY: "https://artifactorycn.netcracker.com/pd.sandbox-staging.go.group,https://artifactorycn.netcracker.com/pd.saas.golang.dev,https://artifactorycn.netcracker.com/pd.saas-release.golang.group"
    # RUN_LOCAL is consumed by Super-Linter inside the common linter image.
    RUN_LOCAL: "true"
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
    # Super-Linter and the test targets need the default branch available for
    # change-detection. GitLab CI checkout does not fetch it by default.
    - git fetch origin ${DEFAULT_BRANCH} --depth=1
    - make ${TARGET}
  rules:
    # Skip branch/tag/schedule pushes and automation MRs.
    - if: '$CI_PIPELINE_SOURCE != "merge_request_event"'
      when: never
    - if: '$CI_MERGE_REQUEST_SOURCE_BRANCH_NAME =~ /^renovate-master/'
      when: never
    - if: '$CI_MERGE_REQUEST_SOURCE_BRANCH_NAME =~ /^fix\/skill-docs-sync/'
      when: never
    - if: '$CI_MERGE_REQUEST_SOURCE_BRANCH_NAME =~ /^chore\/update-apm-packages/'
      when: never
    - when: on_success
```

Import the template from the consumer `.gitlab-ci.yml`:

```yaml
include:
  - project: 'PROD.Platform.Databases/ci-tools'
    ref: <ci-tools-ref>
    file: '/templates/pre-check-common.yml'
```

Or, when the template lives in the same repository:

```yaml
include:
  - local: '/templates/pre-check-common.yml'
```

Then instantiate the base twice — set `TARGET` and any job-specific overrides only:

```yaml
Super Linter:
  tags:
    - OL7
  extends: [.pre-check-base]
  variables:
    TARGET: lint
    # VALIDATE_ALL_CODEBASE is defined in prechecks/custom-liners/super-linter.env.
    # DEFAULT_BRANCH and RUN_LOCAL are inherited from .pre-check-base.

Unit Tests:
  tags:
    - OL7
  extends: [.pre-check-base]
  variables:
    TARGET: test
    PIP_CONFIG_FILE: "$CI_PROJECT_DIR/tests/robot/pip.conf"
    # DEFAULT_BRANCH is inherited from .pre-check-base.
```

Key points:

- `TARGET` must be a Makefile target (`lint`, `test`, or `verify`).
- The entry point must be disabled with `entrypoint: [""]` so the job can run `make`.
- `CI=true` is set automatically by GitLab runners; the consumer Makefile uses it to select the CI branch.
- `RUN_LOCAL=true` is required because Super-Linter is not running inside GitHub Actions.
- The common linter expects the repository at `/tmp/lint`. `run.sh` links `/tmp/lint` to `CI_PROJECT_DIR` when needed.
- The image sets `GOMODCACHE=/tmp/lint/.go/pkg/mod` so CI can cache Go modules inside the repository.
- Fetch `${DEFAULT_BRANCH}` before `make` so Super-Linter and the test skip logic can compare against it.

If Python tests download packages from a private PyPI index, place a `pip.conf` in the repository and set `PIP_CONFIG_FILE` on the unit-test job (or on `.pre-check-base`).

For private Go modules on an internal Git server, add authentication before `make` in `.pre-check-base` `script`, or in a `before_script` on the extending jobs:

```yaml
before_script:
  - git config --global url."https://gitlab-ci-token:${CI_JOB_TOKEN}@git.<private-git-host>.com/".insteadOf "https://git.<private-git-host>.com/"
```

### GitHub Actions

Use the published image in a workflow and disable the entry point. Two jobs in the same workflow run lint and tests in parallel; `make test` inside the common linter container runs Go, Python, and Java tests together:

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
      - name: Configure Maven settings for GitHub Packages
        run: |
          mkdir -p ~/.m2
          cat > ~/.m2/settings.xml <<'EOF'
          <settings>
            <servers>
              <server>
                <id>github</id>
                <username>${{ github.actor }}</username>
                <password>${{ secrets.GITHUB_TOKEN }}</password>
              </server>
            </servers>
          </settings>
          EOF
      - name: Run tests
        run: make test
        env:
          DEFAULT_BRANCH: "master"
          # Set GOPROXY if the default public proxy is unreachable or you need a private module proxy.
          GOPROXY: ${{ vars.GOPROXY }}
          PIP_CACHE_DIR: "${{ github.workspace }}/.pip-cache"
          MAVEN_OPTS: "-Dmaven.repo.local=${{ github.workspace }}/.m2/repository"
```

The `Configure Maven settings for GitHub Packages` step is only needed when Maven dependencies are published to GitHub Packages under a private `netcracker` repository. The `<id>github</id>` must match the repository ID used in the consumer `pom.xml` for `maven.pkg.github.com/netcracker/*`.

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
| `SKILLCHECK_PATTERNS` | Shell globs for skill bundles or individual skills to validate with `skillcheck`. Template default: `agent-packages/*/.apm/skills`. Set overrides in the consumer `Makefile` (or `Makefile.linter`); do not re-set this in the CI job env, because that value wins over Makefile `?=`. Set an empty value to skip skill validation. |

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
