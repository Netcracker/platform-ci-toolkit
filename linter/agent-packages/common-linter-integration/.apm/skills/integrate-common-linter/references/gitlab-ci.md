# GitLab CI integration

GitLab consumers reuse a shared `.pre-check-base` job instead of copying the image and cache blocks. The base is
published in the `ci-tools` project (`templates/pre-check-common.yml`) and extends `.base-netcracker` and
`.run_on_push_and_mr` from `templates/common.gitlab-ci.yml`. Include both files, in that order. Use the same `ref`
the repository already uses for other `ci-tools` includes. This is the shape used by `cloud-backuper`.

```yaml
stages:
  - linter

include:
  - project: 'PROD.Platform.Databases/ci-tools'
    ref: <ci-tools-ref>
    file: '/templates/common.gitlab-ci.yml'
  - project: 'PROD.Platform.Databases/ci-tools'
    ref: <ci-tools-ref>
    file: '/templates/pre-check-common.yml'

Super Linter:
  tags:
    - OL7
  extends: [.pre-check-base]
  variables:
    TARGET: lint

Unit Tests:
  tags:
    - OL7
  extends: [.pre-check-base]
  variables:
    TARGET: test
```

If the repository already includes `common.gitlab-ci.yml`, do not include it twice. If it already has a `linter`
stage, reuse it.

## What `.pre-check-base` provides

Do not redefine these in the consumer; override a single variable only when needed.

- The linter image from the internal registry with `entrypoint: [""]`.
- `DEFAULT_BRANCH` (default `master`), `RUN_LOCAL=true`, `GOPROXY`, `PIP_CACHE_DIR`, `MAVEN_OPTS`.
- Caches for `.go/pkg/mod`, `.pip-cache`, and `.m2/repository`.
- `git fetch origin ${DEFAULT_BRANCH} --depth=1` followed by `make ${TARGET}`.

`TARGET` must be a Makefile target: `lint`, `test`, or `verify`.

## Consumer Makefile differences on GitLab

The wrapper `Makefile` must point `LINTER_IMAGE` at the internal registry mirror used by the repository, so local
`make lint` pulls the same image as CI. Copy the value from `.pre-check-base`; do not guess it.

Set `DEFAULT_BRANCH` in the wrapper `Makefile` when the branch is not `master`. Also set the `DEFAULT_BRANCH` job
variable on both jobs when the base default differs, so the CI fetch step and the Makefile agree.

## Per-job overrides

- Python tests with a private PyPI index: commit a `pip.conf` and set
  `PIP_CONFIG_FILE: "$CI_PROJECT_DIR/<path>/pip.conf"` on the `Unit Tests` job.
- Private Go modules on the internal Git server: add a `before_script` with
  `git config --global url."https://gitlab-ci-token:${CI_JOB_TOKEN}@<git-host>/".insteadOf "https://<git-host>/"`.
- Skip automation merge requests (renovate, APM sync branches) with the same `rules` the repository uses for its
  other MR jobs.

## Replacing existing jobs

If the repository already has a merge request linter job (for example `MR Linter`), keep it. It checks merge request
metadata, not files, and is unrelated to Super-Linter. Ask before deleting any other lint job.
