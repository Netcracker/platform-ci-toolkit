# Supported checks in the common linter

This document lists what the common linter image (`ghcr.io/netcracker/linter`) runs when a consumer calls `make lint`, `make test`, or `make verify`.

The image is based on [GitHub Super-Linter](https://github.com/super-linter/super-linter) v9, plus organization defaults in `super-linter.env`, baseline rule files under `prechecks/base-linters/`, and extra targets in `templates/common.mk` (vendored by consumers as `common.mk`).

For how to run these checks locally or in CI, see [usage.md](usage.md).

---

## What `make verify` runs

```text
make verify
  ├── make lint
  │     ├── Super-Linter          (make lint-super-linter)
  │     ├── skillcheck            (make skill-validate; skipped if SKILLCHECK_PATTERNS is empty)
  │     └── Helm lint + kubeconform (make lint-helm; skipped if no Chart.yaml found)
  └── make test
        ├── test-go
        ├── test-python
        └── test-java
```

CI often splits this into two jobs (`TARGET: lint` and `TARGET: test`) that share the same Makefile targets.

---

## Super-Linter: how enablement works

The baseline is an **allowlist**: `super-linter.env` sets `VALIDATE_*=true` only for the linters listed below, and Super-Linter runs those against matching files in the change set (when `VALIDATE_ALL_CODEBASE=false`). Any linter not listed is off.

Super-Linter does not allow a mix of `VALIDATE_*=true` and `VALIDATE_*=false`. To add a linter in a repo, set `VALIDATE_<NAME>=true` in `prechecks/custom-liners/super-linter.env`. Do **not** set `VALIDATE_<NAME>=false` there; remove it from the allowlist instead.

Important defaults from the image:

| Variable | Image default | Meaning |
| --- | --- | --- |
| `VALIDATE_ALL_CODEBASE` | `false` | Lint files changed against `DEFAULT_BRANCH` (not the whole tree), except for tools that ignore this flag (for example Trivy and Biome in Super-Linter v9). |
| `DEFAULT_BRANCH` | `master` | Comparison branch. Override in the consumer Makefile (`DEFAULT_BRANCH ?= main`); `common.mk` exports it. |
| `FILTER_REGEX_EXCLUDE` | `(^\|/)\.go/pkg/mod/.*` | Skip the Go module cache mounted under the repo. |
| `RUN_LOCAL` | `true` | Required outside GitHub Actions (GitLab and local docker). |

Upstream reference: [Super-Linter environment variables](https://github.com/super-linter/super-linter/blob/main/README.md#configure-super-linter).

---

## Super-Linter: enabled in this image

These are set to `true` in `linter/super-linter.env`. Each runs only when matching files appear in the change set:

| Area | `VALIDATE_*` | Tooling |
| --- | --- | --- |
| Go | `GO`, `GO_MODULES` | golangci-lint (config: `prechecks/base-linters/.golangci.yml`) |
| Java | `JAVA` | Checkstyle (config: `prechecks/base-linters/.checkstyle.xml`). Unit tests still run via `make test-java` in a Maven/JDK image |
| Python | `PYTHON_RUFF` | ruff (config: `prechecks/base-linters/.ruff.toml`) |
| YAML | `YAML` | yamllint config: `prechecks/base-linters/.yaml-lint.yml` |
| Markdown | `MARKDOWN` | Super-Linter default config |
| Dockerfile | `DOCKERFILE_HADOLINT` | hadolint (config: `prechecks/base-linters/.hadolint.yaml`, copy of the Super-Linter default) |
| Renovate | `RENOVATE` | |

Commented out in `super-linter.env` (not active): `VALIDATE_CHECKOV`, `VALIDATE_KUBERNETES_KUBECONFORM`.

For the complete upstream catalog, see [Supported linters](https://github.com/super-linter/super-linter#supported-linters).

---

## Checks outside Super-Linter

These are implemented by `templates/common.mk` / `common.mk` and `run.sh`, not by Super-Linter’s `VALIDATE_*` flags.

### skillcheck (`make skill-validate`)

| Item | Detail |
| --- | --- |
| When | Part of `make lint` when `SKILLCHECK_PATTERNS` is non-empty |
| Default patterns | `agent-packages/*/.apm/skills` |
| Override | Set `SKILLCHECK_PATTERNS` in the consumer Makefile (export is already in the template for that variable) |
| Skip | Set `SKILLCHECK_PATTERNS` to empty |

### Helm lint + kubeconform (`make lint-helm`)

| Item | Detail |
| --- | --- |
| When | Always part of `make lint` |
| Discovery | Every directory under the repository containing a `Chart.yaml`. Dependency charts vendored by `helm dependency update` (nested in another chart's own `charts/` directory) are skipped |
| Steps per chart | `helm lint <chart>`, then `helm template <chart>` piped into `kubeconform -strict -summary -ignore-missing-schemas`, with a `-schema-location` pointing at schemas generated from that chart's own CRDs (if any) |
| Value profiles | If `<chart>/value-profiles/` exists, `helm template` + kubeconform runs once per `*.yaml`/`*.yml` file in it (`-f <profile>`), instead of once with the chart's default `values.yaml`. A chart without that directory is still validated once, against its default values |
| Why kubeconform needs `helm template` first | Super-Linter's own `VALIDATE_KUBERNETES_KUBECONFORM` runs kubeconform against raw files, which breaks on unrendered `{{ ... }}` Go-template syntax in chart templates. Rendering first is required for a meaningful check |
| CRD validation | For every CRD under `<chart>/crds/`, `openapi2jsonschema` (bundled with the `kubeconform` image itself, not built from source) generates a JSON schema, so instances of that chart's own CRDs get structurally validated, not just skipped |
| Why `-ignore-missing-schemas` stays on | Fallback for any kind that still has no schema anywhere — for example a CRD a chart depends on but does not vendor under its own `crds/` |
| Skip | No `Chart.yaml` found anywhere in the repository |
| Override | `HELM_LINT_EXTRA_ARGS`, `HELM_TEMPLATE_EXTRA_ARGS`, `KUBECONFORM_EXTRA_ARGS` env vars, forwarded to the respective command |

### Go tests (`make test-go`)

| Item | Detail |
| --- | --- |
| Discovery | `go.mod` files under `REPO_ROOT` (excluding `vendor/` and `.go/`), or `GO_TEST_MODULES` override |
| Command | `go test ./...` per module that has `*_test.go` |
| Skip | No `.go` / `go.mod` / `.sum` changes against `DEFAULT_BRANCH`, or no modules / no tests |

### Python tests (`make test-python`)

| Item | Detail |
| --- | --- |
| Gate | `PY_BUILD` must be set in the consumer Makefile or the target skips |
| Steps | Run `PY_BUILD` → `pip install pytest` → `PYTHON_TEST_CMD` (default: `$(PYTHON) -m pytest`) |
| Skip | No `.py` changes against `DEFAULT_BRANCH`, or empty `PY_BUILD` |
| Multiple paths | Pass several dirs/packages in `PYTHON_TEST_CMD` |

### Java tests (`make test-java`)

| Item | Detail |
| --- | --- |
| Discovery | `pom.xml` files (excluding `*/target/*`), or `JAVA_TEST_MODULES` override |
| Command | `JAVA_BUILD` (default: `mvn -q test`) in each module |
| Runtime | Common image includes Maven and OpenJDK 25 |
| Skip | No `.java` / `.xml` changes against `DEFAULT_BRANCH`, or no Maven modules |
| Note | Checkstyle (`VALIDATE_JAVA`) lints sources; this target runs the unit tests |

---

## Baseline rule files shipped in the image

Copied into the container work dir by `run.sh` (consumer `prechecks/custom-liners/` can overlay individual files):

| File | Used by |
| --- | --- |
| `.golangci.yml` | Go (`GO` / `GO_MODULES`) |
| `.yaml-lint.yml` | YAML |
| `.ruff.toml` | Python (`PYTHON_RUFF`) |
| `.checkstyle.xml` | Java (`JAVA`, selected via `JAVA_FILE_NAME`) |
| `.hadolint.yaml` | Dockerfile (`DOCKERFILE_HADOLINT`) |

---

## Summary

| Layer | What runs |
| --- | --- |
| Super-Linter | Auto-detect all languages **except** the disables listed in `super-linter.env` |
| skillcheck | Agent Skills under `SKILLCHECK_PATTERNS` |
| Helm lint + kubeconform | Every chart with a `Chart.yaml`, always run, not change-based |
| `test-go` / `test-python` / `test-java` | Language unit tests with change-based skip logic |

Keep `super-linter.env` and this document in sync when adding or removing disables.
