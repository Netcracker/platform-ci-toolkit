# Supported checks in the common linter

This document lists what the common linter image (`ghcr.io/netcracker/linter`) runs when a consumer calls `make lint`, `make test`, or `make verify`.

The image is based on [GitHub Super-Linter](https://github.com/super-linter/super-linter) v9, plus organization defaults in `super-linter.env`, baseline rule files under `prechecks/base-linters/`, and extra targets in `templates/Makefile` (vendored by consumers as `Makefile.precheck`).

For how to run these checks locally or in CI, see [usage.md](usage.md).

---

## What `make verify` runs

```text
make verify
  ├── make lint
  │     ├── Super-Linter          (make lint-super-linter)
  │     └── skillcheck            (make skill-validate; skipped if SKILLCHECK_PATTERNS is empty)
  └── make test
        ├── test-go
        ├── test-python
        └── test-java
```

CI often splits this into two jobs (`TARGET: lint` and `TARGET: test`) that share the same Makefile targets.

---

## Super-Linter: how enablement works

The baseline does **not** set `VALIDATE_*=true`. Super-Linter auto-detects languages from the change set (when `VALIDATE_ALL_CODEBASE=false`) and runs the matching linters.

This image only sets selected `VALIDATE_*=false` values in `super-linter.env`. Everything else stays on Super-Linter’s default (enabled / auto-detect).

Important defaults from the image:

| Variable | Image default | Meaning |
| --- | --- | --- |
| `VALIDATE_ALL_CODEBASE` | `false` | Lint files changed against `DEFAULT_BRANCH` (not the whole tree), except for tools that ignore this flag (for example Trivy and Biome in Super-Linter v9). |
| `DEFAULT_BRANCH` | `master` | Comparison branch. Override in the consumer Makefile (`DEFAULT_BRANCH ?= main`); `Makefile.precheck` exports it. |
| `FILTER_REGEX_EXCLUDE` | `^\.go/pkg/mod/.*` | Skip the Go module cache mounted under the repo. |
| `RUN_LOCAL` | `true` | Required outside GitHub Actions (GitLab and local docker). |

Upstream reference: [Super-Linter environment variables](https://github.com/super-linter/super-linter/blob/main/README.md#configure-super-linter).

---

## Super-Linter: explicitly disabled in this image

These are set to `false` in `linter/super-linter.env`:

| Variable | Notes |
| --- | --- |
| `VALIDATE_CHECKOV` | Infrastructure-as-code scanner |
| `VALIDATE_CSS` | |
| `VALIDATE_HTML` | |
| `VALIDATE_JAVASCRIPT` | Family disable for JS linting |
| `VALIDATE_JSCPD` | Copy-paste detection |
| `VALIDATE_TYPESCRIPT` | Family disable for TS linting |
| `VALIDATE_RUBY` | |
| `VALIDATE_PERL` | |
| `VALIDATE_PHP` | |
| `VALIDATE_KOTLIN` | |
| `VALIDATE_JAVA` | Checkstyle (Java **unit tests** still run via `make test-java`) |
| `VALIDATE_CLOJURE` | |
| `VALIDATE_COFFEESCRIPT` | |
| `VALIDATE_CSHARP` | |
| `VALIDATE_DART` | |
| `VALIDATE_GOOGLE_JAVA_FORMAT` | |
| `VALIDATE_LATEX` | |
| `VALIDATE_LUA` | |
| `VALIDATE_OBJECTIVE_C` | |
| `VALIDATE_R` | |
| `VALIDATE_RUST` | Family disable |
| `VALIDATE_SCALAFMT` | |
| `VALIDATE_SNAKEMAKE` | Family disable |
| `VALIDATE_SQL` | Family disable (includes SQLFluff-style checks) |
| `VALIDATE_TERRAFORM` | Family disable (fmt / tflint) |
| `VALIDATE_XML` | |

To turn one of these back on for a single repo, set it in that repo’s optional `prechecks/custom-liners/super-linter.env`, or pass it as a CI / `docker -e` variable (caller values for `DEFAULT_BRANCH` are restored by `run.sh`; other Super-Linter vars follow normal env-file merge rules — see [usage.md](usage.md)).

---

## Super-Linter: typically active for NetCracker consumers

Any Super-Linter check that is **not** in the disable list above remains eligible. When matching files appear in the change set, these are the ones most often seen in cloud-backuper / dbaas / operator pipelines:

| Area | `VALIDATE_*` (examples) | Tooling |
| --- | --- | --- |
| Go | `GO`, `GO_MODULES`, `GO_RELEASER` | golangci-lint (config: `prechecks/base-linters/.golangci.yml`) |
| Python | `PYTHON_PYLINT`, `PYTHON_FLAKE8`, `PYTHON_BLACK`, `PYTHON_ISORT`, `PYTHON_MYPY`, `PYTHON_RUFF`, `PYTHON_RUFF_FORMAT` | flake8 config: `prechecks/base-linters/.flake8` |
| YAML | `YAML`, `YAML_PRETTIER` | yamllint config: `prechecks/base-linters/.yaml-lint.yml` |
| Markdown | `MARKDOWN`, `MARKDOWN_PRETTIER` | `prechecks/base-linters/.markdownlint.yml` |
| Dockerfile | `DOCKERFILE_HADOLINT` | `prechecks/base-linters/.dockerfilelintrc` |
| Shell | `BASH`, `BASH_EXEC`, `SHELL_SHFMT` | `prechecks/base-linters/.shellcheckrc` |
| JSON | `JSON`, `JSON_PRETTIER`, `JSONC`, … | |
| Git / secrets | `GITLEAKS`, `GIT_MERGE_CONFLICT_MARKERS`, `GIT_COMMITLINT` | Commitlint needs a config file or Super-Linter disables it |
| EditorConfig | `EDITORCONFIG` | |
| Env files | `ENV` | dotenv-linter |
| Biome | `BIOME_FORMAT`, `BIOME_LINT` | May scan more than the git diff (Super-Linter v9 behavior) |
| Trivy | `TRIVY` | May scan more than the git diff; disable per-repo with `VALIDATE_TRIVY=false` if needed |
| GitHub Actions | `GITHUB_ACTIONS`, `GITHUB_ACTIONS_ZIZMOR` | |
| Kubernetes | `KUBERNETES_KUBECONFORM` | |
| Renovate | `RENOVATE` | |
| Natural language | `NATURAL_LANGUAGE` | textlint |
| Spelling | `SPELL_CODESPELL` | |
| OpenAPI / Protobuf / Groovy / PowerShell / Ansible / ARM / CloudFormation / GraphQL / Vue / JSX / TSX / C++ / … | corresponding `VALIDATE_*` | Only if those files are present and not covered by a family disable above |

For the complete upstream catalog, see [Supported linters](https://github.com/super-linter/super-linter#supported-linters).

---

## Checks outside Super-Linter

These are implemented by `templates/Makefile` / `Makefile.precheck` and `run.sh`, not by Super-Linter’s `VALIDATE_*` flags.

### skillcheck (`make skill-validate`)

| Item | Detail |
| --- | --- |
| When | Part of `make lint` when `SKILLCHECK_PATTERNS` is non-empty |
| Default patterns | `agent-packages/*/.apm/skills` |
| Override | Set `SKILLCHECK_PATTERNS` in the consumer Makefile (export is already in the template for that variable) |
| Skip | Set `SKILLCHECK_PATTERNS` to empty |

### Go tests (`make test-go`)

| Item | Detail |
| --- | --- |
| Discovery | `go.mod` files under `REPO_ROOT` (excluding `vendor/`), or `GO_TEST_MODULES` override |
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
| Note | Super-Linter’s `VALIDATE_JAVA` is **off**; this target is the supported Java check |

---

## Baseline rule files shipped in the image

Copied into the container work dir by `run.sh` (consumer `prechecks/custom-liners/` can overlay individual files):

| File | Used by |
| --- | --- |
| `.golangci.yml` | Go (`GO` / `GO_MODULES`) |
| `.flake8` | Python flake8 |
| `.markdownlint.yml` | Markdown |
| `.yaml-lint.yml` | YAML |
| `.shellcheckrc` | ShellCheck |
| `.dockerfilelintrc` | Hadolint |

---

## Summary

| Layer | What runs |
| --- | --- |
| Super-Linter | Auto-detect all languages **except** the disables listed in `super-linter.env` |
| skillcheck | Agent Skills under `SKILLCHECK_PATTERNS` |
| `test-go` / `test-python` / `test-java` | Language unit tests with change-based skip logic |

Keep `super-linter.env` and this document in sync when adding or removing disables.
