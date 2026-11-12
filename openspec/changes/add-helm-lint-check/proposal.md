# Proposal

## Why

The common linter image (Super-Linter + skillcheck) has no check for Helm charts. Super-Linter's own `VALIDATE_KUBERNETES_KUBECONFORM` cannot be used for this: it runs kubeconform directly against raw files, and a chart's `templates/*.yaml` still contains unrendered `{{ ... }}` Go-template syntax at that point, which breaks YAML parsing before kubeconform can even check a schema. Consumers (e.g. `cloud-backuper`) ship Helm charts and want `helm lint` plus real validation of the rendered manifests as part of `make lint`.

## What Changes

- New `lint-helm` Makefile target (`linter/templates/common.mk`), wired into `make lint` alongside `lint-super-linter` and `skill-validate`, in both the CI and local invocation modes.
- New `helm-lint` command in `linter/run.sh` (`run_helm_lint`), dispatched the same way as the existing `super-linter` and `skillcheck` commands:
  - Discovers every directory containing a `Chart.yaml` under `/tmp/lint` (`find -L`, required because this image's `find` is BusyBox and does not follow the `/tmp/lint` symlink created in CI without `-L`).
  - Skips dependency charts vendored by `helm dependency update` (a `Chart.yaml` nested inside another chart's own `charts/` directory) so they are not linted twice — but only when the parent directory is literally named `charts` **and** that parent's own parent has a `Chart.yaml`, so charts that merely live in a directory named `charts` (e.g. `services/foo/deployments/charts/foo`) are still linted.
  - Runs `helm lint <chart>` on each remaining chart.
  - Renders each chart with `helm template` and validates the output with `kubeconform -strict`.
  - For every CRD under `<chart>/crds/`, generates a JSON schema with `openapi2jsonschema` and adds it as a `kubeconform -schema-location`, so instances of the chart's own CRDs are structurally validated instead of being skipped. `-ignore-missing-schemas` stays on as a fallback for any other kind kubeconform still does not know.
  - If `<chart>/value-profiles/` exists, renders and validates the chart once per values file found there instead of once with the chart's default `values.yaml`. A chart without that directory keeps the single default-values run.
- `linter/Dockerfile`: add a build stage that copies `/openapi2jsonschema` from the already-pinned `ghcr.io/yannh/kubeconform` image. `helm` and `kubeconform` themselves already ship in the Super-Linter base image; `openapi2jsonschema` is bundled in the same upstream `kubeconform` image but dropped by Super-Linter's own Dockerfile, so this is a second `COPY --from=` against an image already referenced elsewhere in the build — not a new build-from-source step.
- Documentation sync: `linter/README.md`, `linter/docs/supported-checks.md`, `linter/AGENTS.md` updated to describe `lint-helm` and its CRD-schema generation.

## Capabilities

### New Capabilities
- `helm-lint-check`: running `helm lint` and a rendered-manifest `kubeconform` check (with CRD-aware schema generation) against every Helm chart in a consumer repository, as part of the common linter's `make lint`.

### Modified Capabilities

(none — this is the first capability recorded for this project)

## Impact

- `linter/Dockerfile`: one additional `COPY --from=` build stage; no new runtime dependency beyond what the image already pulls.
- `linter/run.sh`: new `run_helm_lint` function and `helm-lint` dispatch case, also wired into the default `all` command.
- `linter/templates/common.mk`: new `lint-helm` target in both the CI (`ifeq ($(CI),true)`) and local branches; consumers who vendor this file as `common.mk`/`Makefile.precheck` need to re-sync it to pick up the new target.
- Consumer repositories with Helm charts (e.g. `cloud-backuper`) will see `make lint` take longer and potentially fail on pre-existing chart issues once they pick up the updated image and template — this was observed directly: `cloud-backuper`'s `app-chart` has an invalid `Chart.yaml` `apiVersion`, and its `velero` chart has a duplicate `type` key in a rendered `Service`, both previously unreported.
