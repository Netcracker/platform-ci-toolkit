# How Super-Linter handles Go

> **Current behavior:** The common linter image now relies on Super-Linter’s built-in `GO_MODULES` check. The custom per-module Go linter described in the historical sections below has been removed. `make lint` runs Super-Linter only; `make lint-filter FILTER=...` can narrow the scope to a specific module or folder.

This page describes how GitHub Super-Linter v9 runs Go linters, why its `GO_MODULES` mode can time out on multi-module repositories such as Cloud Backuper, and which alternative approaches were tried and rejected.

## How Super-Linter dispatches linters

Super-Linter scans the repository, builds a list of files to check, and runs one or more linters against each file based on its extension. For Go it supports two modes:

| Mode | When it runs | Command |
| --- | --- | --- |
| `GO` | `.go` files are found and there is no root `go.mod` | `golangci-lint run -c .golangci.yml --fast-only <files>` |
| `GO_MODULES` | A `go.mod` file exists anywhere in the repository | `golangci-lint run --allow-parallel-runners -c .golangci.yml` |

Source: [`super-linter/lib/functions/linterCommands.sh`](https://github.com/super-linter/super-linter/blob/main/lib/functions/linterCommands.sh)

## Why `GO_MODULES` times out on Cloud Backuper

Cloud Backuper has no `go.mod` at the repository root. Instead it has 11 independent Go modules under `services/`:

```text
services/backuper-service/go.mod
services/trimmer-velero/go.mod
services/gcp-parallel-snapshotter/go.mod
services/velero-plugins/velero-netcracker-plugins-common-lib/go.mod
services/velero-plugins/velero-netcracker-plugin-for-gcp/go.mod
services/velero-plugins/velero-netcracker-plugin-for-openstack/go.mod
services/velero-plugins/velero-netcracker-mutator-plugin/go.mod
services/velero-plugins/velero-netcracker-aws-aurora-plugin/go.mod
services/velero-plugins/velero-gcp-sql-plugin/go.mod
services/velero-plugins/velero-netcracker-plugin-for-azure/go.mod
```

Because at least one `go.mod` exists, Super-Linter enables `GO_MODULES` and runs this command from `/tmp/lint` (the repository root):

```bash
golangci-lint run --allow-parallel-runners -c .golangci.yml
```

`golangci-lint` with no path argument loads packages from the current directory. The repository root has no `go.mod`, so it walks the entire tree and tries to analyze all 11 modules and their dependencies at once. The resulting package graph is too large and the linter exits with a timeout or `context deadline exceeded` error.

## Why `GO` mode is not a fix

The `GO` linter passes individual files to `golangci-lint`:

```bash
golangci-lint run -c .golangci.yml --fast-only file1.go file2.go ...
```

`golangci-lint` requires every file on the command line to be in the same directory. Cloud Backuper has `.go` files in many directories, so `GO` mode fails with:

```text
named files must all be in one directory
```

See [super-linter/super-linter#6294](https://github.com/super-linter/super-linter/issues/6294).

## What this image does now

`run.sh` runs Super-Linter directly. Go linting is handled by Super-Linter’s built-in `GO` / `GO_MODULES` checks, just like any other language.

The consumer `Makefile` provides these targets:

```bash
make lint                                # Super-Linter on changed files
make lint-filter FILTER=services/backuper-service/.*  # Super-Linter on a filtered set
make lint-fix                            # Super-Linter with auto-fix enabled
```

Because `VALIDATE_ALL_CODEBASE=false` is set by default, Super-Linter only lints changed files and the Go modules that contain those changed files. To lint a single module regardless of what changed, use `make lint-filter FILTER=services/backuper-service/.*`.

### Historical note: custom per-module Go linter

Earlier versions of this image added a custom per-module Go linter to avoid the whole-repository `GO_MODULES` timeout described above. It ran `golangci-lint` separately for each module:

```bash
cd services/backuper-service && golangci-lint run --config .golangci.yml ./...
cd services/trimmer-velero && golangci-lint run --config .golangci.yml ./...
# ... one run per module
```

This approach was later removed to keep the common linter aligned with standard Super-Linter behavior and to reduce maintenance overhead.

`lint_go_modules` respects `FILTER_REGEX_INCLUDE`, so `make lint-module MODULE=...` limits linting to a single service. When no filter is set, it only lints modules that contain `.go` files changed since `DEFAULT_BRANCH`.

You can disable the custom Go runner by setting `PLATFORM_CI_CUSTOM_GO_LINT=false`. In that case Super-Linter's `GO_MODULES` runs as usual and will likely time out on multi-module repositories.

## Trials and rejected alternatives

### Generating a `go.work` file and using Super-Linter's `GO_MODULES`

A natural fix is to generate a temporary `go.work` file that includes every module, warm the Go cache, and let Super-Linter's `GO_MODULES` run `golangci-lint` once. This approach was implemented and tested against Super-Linter v6, but it failed because the Go toolchain inside v6 (1.22.3 at the time of testing) was older than the one Cloud Backuper requires. The `go` command could not parse `services/backuper-service/go.mod`:

```text
go: errors parsing services/backuper-service/go.mod:
services/backuper-service/go.mod:238: unknown directive: tool
```

Cloud Backuper uses the `tool` directive and declares `go 1.27.1`. The current image is based on Super-Linter v9, which ships a Go 1.27.1 toolchain, so the parse error is gone. However, generating a workspace still does not solve the scope problem: Super-Linter's `GO_MODULES` runs `golangci-lint` from the repository root and tries to load every module at once, which times out on large repositories. The custom per-module runner remains the chosen approach.

### Changing `DEFAULT_WORKSPACE` per module

`DEFAULT_WORKSPACE` tells Super-Linter where the checkout is, but Super-Linter still initializes every linter, scans repository metadata, and runs `golangci-lint` from the same root. A test run with `DEFAULT_WORKSPACE` set to a single module reached the 10-minute timeout while still gathering the file list, so this approach is not practical.

### Pre-populating the Go cache alone

Pre-warming `GOMODCACHE` and `GOCACHE` removes the time spent downloading dependencies, but it does not change the scope problem: Super-Linter's `GO_MODULES` still runs `golangci-lint` from the repository root without a valid workspace, so it still tries to load all modules at once. The cache is therefore not a replacement for the custom per-module runner.

### Persisting a `.go-cache` directory inside the repository mount

A `.go-cache/` directory inside the mounted repository was tried so CI systems could persist it between jobs. The first implementation used `find ... -not -path '*/.go-cache/*'` to skip the cache, but `find` still descended into the cache directory and the cache also contained `go.mod` files from downloaded dependencies. On Cloud Backuper the search reported 152 modules instead of 10. Using `find -prune` and adding `.go-cache/` to `FILTER_REGEX_EXCLUDE` fixed the search, but the approach still added complexity and a directory that could be committed by accident, so it was removed in favor of running `lint_go_modules` directly without a shared cache.

## Private modules and credentials

Even with the correct Go toolchain, the custom Go linter cannot analyze modules that depend on private repositories unless `go` can fetch them. Without credentials, `go` blocks while waiting for a username; the current runner uses a 15-minute `golangci-lint` timeout so the job fails instead of hanging. The usual fixes are:

- In GitLab CI, rewrite `https://git.netcracker.com/` URLs to use `gitlab-ci-token:${CI_JOB_TOKEN}` before running `make lint`.
- Mount a pre-populated `go/pkg/mod` cache that already contains the required private-module versions.
- Provide a `.netrc` or equivalent token inside the container.
