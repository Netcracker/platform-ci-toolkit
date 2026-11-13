# Tasks

## 1. Image: bundle `openapi2jsonschema` without building from source

- [x] 1.1 Add `ARG OPENAPI2JSONSCHEMA_IMAGE` (pinned `ghcr.io/yannh/kubeconform:v0.8.0@sha256:...`) and a `FROM ${OPENAPI2JSONSCHEMA_IMAGE} AS openapi2jsonschema` stage to `linter/Dockerfile`, then `COPY --from=openapi2jsonschema /openapi2jsonschema /usr/bin/openapi2jsonschema`. Verify by building the image and running `docker run --rm --entrypoint openapi2jsonschema <image>`, which must print its `Usage: openapi2jsonschema [FILE]` message rather than "not found".

## 2. `run.sh`: chart discovery, `helm lint`, and rendered-manifest validation

- [x] 2.1 Add `run_helm_lint` to `linter/run.sh`: discover every `Chart.yaml` under `/tmp/lint` with `find -L` (required because this image's `find` is BusyBox, which does not follow the `/tmp/lint` symlink used in CI without `-L`), skip chart directories found to be vendored dependencies (parent directory named `charts` **and** that parent's own parent has a `Chart.yaml`), and run `helm lint` on every remaining chart. Verify by running it against a repository with multiple real charts (including one under a path that merely contains a directory literally named `charts`) and confirming all of them are found and linted, none of the genuinely vendored ones are double-linted, and a chart with an invalid `Chart.yaml` is reported as a failure.
- [x] 2.2 Render each chart with `helm template` into a `mktemp --suffix=.yaml` file (plain `mktemp`, with no extension, makes kubeconform silently report zero files) and validate it with `kubeconform -strict -summary -ignore-missing-schemas`. On a `helm template` failure, print its stderr and fail that chart without calling kubeconform. Verify by running against a chart with a real rendered-manifest structural error (e.g. a duplicate key produced by template logic) and confirming kubeconform reports it and the check fails, while a clean chart reports all resources valid.
- [x] 2.3 Add the `helm-lint` case to the `run.sh` command dispatcher (alongside `super-linter` and `skillcheck`) and include it in the `all` case's pass/fail aggregation. Verify with `run.sh helm-lint` directly and with `run.sh all`, confirming a chart failure makes the exit code non-zero in both.

## 3. `run.sh`: validate instances of a chart's own CRDs

- [x] 3.1 In `run_helm_lint`, for each chart with a `<chart>/crds/` directory, run `openapi2jsonschema` against every YAML file there from inside a per-chart `mktemp -d` scratch directory (the tool writes `{kind}_{version}.json` to its current directory), and pass that directory as an extra `-schema-location` to kubeconform (before the existing `-ignore-missing-schemas` fallback). Treat a schema-generation failure for one CRD as a logged warning, not an abort. Verify against a chart that both ships a CRD under `crds/` and renders an instance of it (e.g. Velero's chart and its `VolumeSnapshotLocation`): the instance must move from "Skipped" (no schema) to actually validated, and a deliberately malformed instance of that same kind must be reported as invalid.

## 4. `common.mk`: wire `lint-helm` into `make lint`

- [x] 4.1 Add a `lint-helm` target to both the CI (`ifeq ($(CI),true)`) and local branches of `linter/templates/common.mk`, calling `run.sh helm-lint` directly in CI and via `$(LINTER_RUN) helm-lint` locally, and add it to the `lint:` target's dependencies alongside `lint-super-linter` and `skill-validate`. Verify `make lint-helm` succeeds standalone in both local mode (bind mount) and a CI-mode simulation (`CI=true`, repository mounted away from `/tmp/lint`, `/tmp/lint` created as a symlink the way `prepare-ci-workspace` does it, container entrypoint disabled), with matching chart-discovery and validation results in both modes.

## 5. Documentation

- [x] 5.1 Update `linter/README.md`'s command list, `linter/docs/supported-checks.md` (the `make verify` tree, a dedicated "Helm lint + kubeconform" section, the baseline-checks summary table), and `linter/AGENTS.md` (layout description and the "Rules that are not obvious from the code" list: the BusyBox `find -L` requirement, the `mktemp --suffix=.yaml` requirement, and where the CRD schemas come from) to describe `lint-helm` and its CRD-schema generation. Verify by re-reading each updated section against the actual `run_helm_lint` behavior for accuracy.

## 6. End-to-end verification against a real consumer repository

- [x] 6.1 Run the full `make lint-helm` (and `run.sh all`) against a real multi-chart consumer repository in both local and CI-mode simulation, and confirm: all charts are discovered, pre-existing real chart issues are surfaced (not just synthetic test fixtures), a chart's own CRD instances are validated rather than skipped, and the process exits non-zero exactly when a real issue is present.

## 7. `run.sh`: validate every value profile a chart ships

- [x] 7.1 In `run_helm_lint`, discover `<chart>/value-profiles/*.yaml` (and `.yml`); when present, run the existing render-and-validate step once per profile (`helm template -f <profile>`) instead of once with default values, labeling each run by its profile file so a failure is attributable to a specific profile; when absent, keep the single default-values run unchanged. Reuse the chart's CRD schema directory (generated once per chart) across all of that chart's profiles. Verify by temporarily giving a real chart a `value-profiles/` directory with multiple real-world values files and confirming: every profile is discovered and run, a profile whose values fail `helm template` is reported against that profile without aborting the other profiles, a profile that renders is still validated by kubeconform (including against the CRD schema path), and a chart without the directory still gets exactly one default-values run as before.
