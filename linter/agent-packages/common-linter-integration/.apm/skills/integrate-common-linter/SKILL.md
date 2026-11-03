---
name: integrate-common-linter
description: Integrate the platform-ci-toolkit common linter (Super-Linter plus skillcheck) into a consumer repository, locally and in GitHub Actions or GitLab CI. Use when adding, migrating, or debugging `make lint`, `make test`, `Makefile.common`, or a lint/test CI job.
---

# Integrate the common linter into a consumer repository

The common linter image (`ghcr.io/netcracker/linter`) bundles Super-Linter and `skillcheck`. Language tests run in
language-specific test images. A consumer repository does not configure linters directly. It vendors one Makefile and runs `make lint` and
`make test` identically on a laptop and in CI. Upstream docs: `linter/docs/usage.md` and
`linter/docs/supported-checks.md` in `Netcracker/platform-ci-toolkit`.

## Workflow

1. Detect the host. Read the `origin` remote and look for `.gitlab-ci.yml` or `.github/workflows/`.
   Then read the matching reference: [GitHub Actions](references/github-actions.md) or
   [GitLab CI](references/gitlab-ci.md).
2. Detect the repository facts that feed the Makefile overrides (see the table below).
3. Vendor `Makefile.common` and write the thin wrapper `Makefile` (next section).
4. Add or update the CI jobs from the reference. Reuse the existing CI structure; do not replace it.
5. Verify with `make lint` locally (needs Docker), then confirm the CI jobs by reading their logs.

## Consumer Makefile

`Makefile.common` is an unmodified copy of `linter/templates/Makefile.common.mk`. Never edit it; refresh it with:

```bash
make sync-precheck-makefile PLATFORM_CI_TOOLKIT_RAW_URL=<raw-url-to-linter/templates/Makefile.common.mk>
```

If the repository already has a `Makefile`, keep its existing targets and add the overrides above the `include`.
Put every repo-specific value in the wrapper:

```make
LINTER_IMAGE ?= ghcr.io/netcracker/linter
LINTER_TAG ?= latest
DEFAULT_BRANCH ?= main
include Makefile.common
```

## Detect before overriding

| Fact | How to detect | Override |
| --- | --- | --- |
| Default branch | `git symbolic-ref refs/remotes/origin/HEAD`; image default is `master` | `DEFAULT_BRANCH` |
| Skill locations | `find . -path '*/.apm/skills' -not -path './apm_modules/*'`; the template already covers the standard APM package layout | `SKILLCHECK_PATTERNS`, then `export` it |
| Go modules | `go.mod` files; auto-detected | `GO_TEST_MODULES` only to narrow (e.g. exclude test apps) |
| Python tests | `requirements*.txt` next to tests; tests are skipped unless set | `PY_BUILD` (installs deps, may set `PYTHONPATH`) |
| Maven modules | `pom.xml` files; auto-detected | `JAVA_TEST_MODULES`, `JAVA_BUILD` |
| Private Go modules | private hosts in `go.mod` | `GOPROXY` or git `insteadOf` auth in CI |

## Rules that are easy to get wrong

- Do not set `SKILLCHECK_PATTERNS` in CI variables or workflow `env`. An environment value wins over the Makefile
  `?=` and silently drops paths. Set it only in the wrapper `Makefile`.
- The CI job must run inside the linter image with the entrypoint disabled (`entrypoint: [""]` on GitLab,
  `options: --entrypoint=""` on GitHub), otherwise `make` is never reached.
- GitHub checkout needs `fetch-depth: 0`. On GitLab, run `git fetch origin ${DEFAULT_BRANCH} --depth=1` before
  `make`. Without the default branch ref, Super-Linter cannot compute changed files and the test targets run
  everything.
- `RUN_LOCAL=true` is required outside the Super-Linter GitHub Action, in every CI system.
- `CI=true` selects the in-container recipes in `Makefile.common`. Both GitHub and GitLab set it; never set it
  locally.
- Use `make lint` and `make test` as separate CI jobs so they run in parallel. `make verify` runs both serially.
- Keep Go, pip and Maven caches inside the repository (`.go/pkg/mod`, `.pip-cache`, `.m2/repository`) so CI cache
  steps can save them.
- Add the caches and `Makefile`-generated directories (`.go/`, `.pip-cache/`, `.m2/`) to `.gitignore`.
- Pin the image by a tag the repository controls. Use `latest` unless the toolkit team asks for a branch tag.

## Verification

- `make lint` prints `=== Running Super-Linter ===` and, when skills exist, `=== Running skill validation ===`.
- `make test` prints one `[test-*]` section per language and skips languages with no changed files.
- If lint reports `The workspace (/tmp/lint) is not a directory!`, the image is outdated; pull the current tag.
- If a large multi-module repository times out, narrow with `make lint FILTER_REGEX_INCLUDE=<module>/.*`.
