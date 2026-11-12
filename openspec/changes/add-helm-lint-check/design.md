# Design

## Context

See `proposal.md` - Why. The common linter image is built from `ghcr.io/super-linter/super-linter:slim-v9` (`linter/Dockerfile`), driven by `linter/run.sh` (the container entrypoint, dispatches `super-linter` / `skillcheck` / `all`), consumed by `linter/templates/common.mk` (vendored by consumers as `common.mk`, with a CI branch that runs inside the already-running container and a local branch that wraps it in `docker run`). Both `helm` and `kubeconform` already ship in the Super-Linter base image (`alpine/helm:4.3.0` and `ghcr.io/yannh/kubeconform:v0.8.0`, copied in by Super-Linter's own `Dockerfile`), so no language runtime needs to be added for the base check.

## Goals / Non-Goals

**Goals:**
- Add a Helm chart check that works identically whether `common.mk` runs locally (direct bind mount at `/tmp/lint`) or in CI (symlink to the real checkout, created by `prepare-ci-workspace` or by `run.sh`'s own fallback).
- Validate what a chart actually renders, not its raw template source.
- Validate instances of a chart's own CRDs structurally, not just skip them.
- Avoid adding a new build-from-source step to the image for this.

**Non-Goals:**
- Validating CRDs a chart depends on but does not vendor under its own `crds/` (handled by falling back to skip, not by fetching schemas from elsewhere).
- Chart-level policy/security checks (that is Checkov's job, tracked separately - not part of this change).
- Making the check incremental/diff-based. `VALIDATE_ALL_CODEBASE=false` lets Super-Linter skip unchanged files; this check does not participate in that, it always runs against every chart.

## Decisions

### Dispatch as a third `run.sh` command, not inline in the Makefile
`run.sh` already dispatches `super-linter` and `skillcheck` as named commands behind a `case`, with `all` running everything and returning non-zero if any step failed. Adding `helm-lint` as a third case keeps chart-discovery and validation logic in one place (`run.sh`), with `common.mk`'s `lint-helm` target (CI and local branches, mirroring `lint-super-linter`) staying a thin dispatcher. The alternative - writing the discovery/validation loop directly in the Makefile recipe - was rejected because the existing two checks already use this split, and duplicating non-trivial shell logic in `make` recipes (which already need `$$`-escaping for `$()`) is harder to read and test than a function in `run.sh`.

### `find -L`, because this image's `find` is BusyBox
Tested directly: `find /tmp/lint -name Chart.yaml` returns 0 results when `/tmp/lint` is a symlink (the CI case), but finds every chart when it is a real bind mount (the local case) - BusyBox `find` does not follow a symlinked starting path without `-L`, unlike GNU `find`. Without `-L`, the check would have silently found zero charts and reported success in every real CI run while still appearing to work locally. `run_helm_lint` uses `find -L`. (`warn_if_missing_go_proxy`'s existing `find /tmp/lint -name go.mod` has the same latent gap and was left alone - unrelated to this change.)

### `mktemp --suffix=.yaml` for the rendered file
kubeconform filters input files by extension; a plain `mktemp` file (no extension) made it silently report "0 resource found in 0 file" instead of validating anything - caught by testing the rendered output against a real chart, not from reading kubeconform's docs. Fixed by giving the rendered-manifest temp file a `.yaml` suffix.

### CRD schemas for validation come from the `kubeconform` image itself, not a build step
kubeconform's companion tool `openapi2jsonschema` (converts a CRD's embedded OpenAPI schema into a JSON schema kubeconform can validate against) is not published as a standalone binary or Docker image, and `go install github.com/yannh/kubeconform/scripts/go@latest` does not work (that module's `go.mod` declares a path that does not match where the directory actually lives in the repo). The alternative considered was cloning `yannh/kubeconform` and running `go build` in a dedicated build stage - it works (verified locally) but adds a clone + Go build step to the image for a ~9 MB binary. Inspecting `yannh/kubeconform`'s own `Dockerfile` (driven by its `.goreleaser.yml`, which has a separate `openapi2jsonschema` build alongside `kubeconform`) showed that the published `ghcr.io/yannh/kubeconform` image already contains `/openapi2jsonschema` right next to `/kubeconform` - Super-Linter's own `Dockerfile` only copies `/kubeconform` from that image into its base image. `linter/Dockerfile` adds a second `FROM ghcr.io/yannh/kubeconform:v0.8.0@sha256:... AS openapi2jsonschema` stage (same image, pinned by digest for reproducibility, matching how `SKILLCHECK_IMAGE` is already pinned) and copies `/openapi2jsonschema` from it. No cloning, no `go build`, no new toolchain.

### CRD schema generation is scoped per chart, from that chart's own `crds/`
`openapi2jsonschema` always writes `{kind}_{version}.json` to its current working directory, so it is run from inside a per-chart scratch directory (`mktemp -d`), once per YAML file under `<chart>/crds/`, before that chart is validated; the directory is passed to kubeconform as an extra `-schema-location` (after `default`) and removed after that chart's validation. Scoping per chart (rather than scanning the whole repository once) matches how charts in the repositories inspected so far ship their own CRDs alongside the resources that use them (for example Velero's chart ships `crds/volumesnapshotlocations.yaml` and also renders a `VolumeSnapshotLocation`), and keeps a chart's validation self-contained. `-ignore-missing-schemas` stays on kubeconform's invocation as a fallback, so a kind with no schema anywhere (built-in catalog or this chart's own CRDs) is reported as skipped, not a hard failure.

### `value-profiles/` replaces the default-values run, it does not add to it
A chart's default `values.yaml` is frequently a set of incomplete placeholders (empty strings for required fields such as a namespace or image repository), not something anyone actually deploys, so validating only against it has limited value. When `<chart>/value-profiles/*.yaml` (or `.yml`) exists, each profile is rendered and validated as its own `helm template -f <profile>` + kubeconform run instead of the single default-values run, since the profiles are the actual deployment configurations meant to be exercised. The CRD schema directory is still generated once per chart (schemas do not depend on values) and reused across every profile of that chart. A chart without the directory keeps exactly the previous single-run behavior, so this is purely additive for charts that do not use the convention.

### Vendored-dependency detection checks grandparent, not a path substring
An earlier draft excluded any chart whose path contained `charts/` to avoid double-linting dependencies vendored by `helm dependency update`. Tested against `cloud-backuper`: that would have wrongly excluded 3 of its 8 real charts, which live under paths like `services/foo/deployments/charts/foo` where `charts` is just a directory name, not a vendored dependency. The check now only treats a chart as a vendored dependency when its parent directory is named `charts` **and** that parent's own parent also has a `Chart.yaml` - i.e. it is nested inside another chart's dependency folder.

## Risks / Trade-offs

- **[Risk]** A chart with many resources and large CRDs could make `helm template` + schema generation + `kubeconform` noticeably slower than the rest of `make lint`. → Not change-based by design (every chart is always checked, see Non-Goals); if this becomes a problem, `HELM_LINT_EXTRA_ARGS` / `HELM_TEMPLATE_EXTRA_ARGS` / `KUBECONFORM_EXTRA_ARGS` give consumers a way to narrow scope without changing the image.
- **[Risk]** `openapi2jsonschema` failing on a malformed CRD could abort the whole check. → `run_helm_lint` treats a failed schema generation as a warning (logged, continues with whatever schemas it did generate) rather than aborting that chart's check.
- **[Risk]** Pinning `ghcr.io/yannh/kubeconform` by digest means a future Super-Linter base image bump to a newer kubeconform version will not automatically bump the digest used for `openapi2jsonschema`, so the two could drift apart. → Acceptable for now (both are from the same upstream release today); revisit if Super-Linter's bundled kubeconform version moves.
- **[Trade-off]** Consumers who vendor `common.mk` as a static copy (per `docs/usage.md`) will not get `lint-helm` until they re-sync it; this was already true for any template change, not new to this feature.

## Migration Plan

- No runtime migration: this only adds a new check. The image gains a new stage (`openapi2jsonschema`) and `run.sh` gains a new dispatch case; existing `super-linter` / `skillcheck` / `all` behavior is unchanged except that `all` now also runs `helm-lint`.
- Consumers pick this up when they bump `LINTER_TAG` to an image built from this change and re-sync `common.mk` from `linter/templates/common.mk`.
- Rollback: revert the image tag / template sync; no state to clean up.
