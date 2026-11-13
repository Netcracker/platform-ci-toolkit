# Spec Delta

## Purpose

Lints and validates every Helm chart in a consumer repository as part of the common linter's `make lint`, catching chart issues that Super-Linter's own Kubernetes check cannot see because it runs against unrendered chart templates.

## ADDED Requirements

### Requirement: Chart discovery
The system SHALL discover every Helm chart in the repository by finding every directory that contains a `Chart.yaml`, regardless of whether the repository root is mounted directly or reached through a symlink.

#### Scenario: Multiple charts in different subdirectories
- **WHEN** a repository contains several charts in different subdirectories (for example under both `services/<name>/deployments/charts/<name>` and `services/<name>/helm-templates/<name>`)
- **THEN** every one of those chart directories is discovered and processed

#### Scenario: Repository root reached through a symlink
- **WHEN** the repository is mounted at a path and made reachable through a symlink to that path, as happens in CI
- **THEN** chart discovery still finds every `Chart.yaml` under the symlinked path, the same as when the path is a direct mount

#### Scenario: No charts in the repository
- **WHEN** the repository contains no `Chart.yaml` anywhere
- **THEN** the check reports that no charts were found and succeeds without running `helm lint` or kubeconform

### Requirement: Vendored dependency charts are not linted standalone
The system SHALL skip a discovered chart when it is a dependency vendored into another chart by `helm dependency update`, so it is not linted twice (once as part of its parent, once on its own). A chart is treated as a vendored dependency only when its parent directory is named `charts` **and** that parent's own parent directory also contains a `Chart.yaml`.

#### Scenario: Chart vendored under another chart's charts/ directory
- **WHEN** a chart's `Chart.yaml` is found at `<parent-chart>/charts/<dependency>/Chart.yaml`, and `<parent-chart>/Chart.yaml` exists
- **THEN** `<dependency>` is skipped as a standalone chart and reported as a skipped vendored dependency

#### Scenario: Chart directory literally named "charts" that is not a dependency
- **WHEN** a chart lives under a path where an ancestor directory is named `charts` for organizational reasons (for example `services/foo/deployments/charts/foo`), and that `charts` directory's parent does not itself have a `Chart.yaml`
- **THEN** the chart is still discovered and linted, not skipped

### Requirement: `helm lint` runs on every discovered chart
The system SHALL run `helm lint` against every discovered, non-vendored chart and SHALL cause the overall check to fail if any chart fails `helm lint`.

#### Scenario: Chart with an invalid Chart.yaml
- **WHEN** a chart's `Chart.yaml` is missing a required field (for example `apiVersion`)
- **THEN** `helm lint` reports the error for that chart and the overall Helm lint check fails

### Requirement: Rendered manifests are validated, not raw templates
The system SHALL render each chart with `helm template` before validating it, and SHALL validate the rendered output, not the chart's raw template source, because raw templates contain unrendered template syntax that is not valid YAML on its own.

#### Scenario: Chart template contains Go template conditionals
- **WHEN** a chart's template files contain `{{ ... }}` directives that are not valid YAML by themselves (for example a standalone `{{- if ... }}` line)
- **THEN** the check renders the chart first and validates the rendered output, and does not fail merely because the raw template file is not valid YAML

#### Scenario: Rendering fails
- **WHEN** `helm template` fails for a chart (non-zero exit)
- **THEN** the check reports the render failure with its error output and fails the overall check for that chart, without attempting to validate a partial or empty render

### Requirement: Every value profile a chart ships is rendered and validated
The system SHALL treat `<chart>/value-profiles/` as the standard location for a chart's deployment value profiles. When that directory contains one or more values files, the system SHALL render and validate the chart once per values file found there, in place of a single run against the chart's default `values.yaml`. When that directory does not exist or is empty, the system SHALL render and validate the chart once, against its default `values.yaml`, as it would with no profiles at all.

#### Scenario: Chart ships multiple value profiles
- **WHEN** a chart has a `value-profiles/` directory containing several values files (for example one per cloud provider)
- **THEN** `helm template` + kubeconform validation runs once per values file in that directory, and a failure in any one profile is reported against that profile without skipping the remaining profiles

#### Scenario: Chart has no value-profiles directory
- **WHEN** a chart has no `value-profiles/` directory (or an empty one)
- **THEN** the chart is rendered and validated exactly once, against its default `values.yaml`, the same as before this requirement existed

### Requirement: Rendered manifests are schema-validated with kubeconform
The system SHALL validate each chart's rendered manifests against Kubernetes schemas and SHALL fail the check when a rendered resource is invalid against a schema it has.

#### Scenario: Rendered manifest has a structural error
- **WHEN** a chart's rendered output contains a resource with a structural error (for example the same key set twice in one mapping)
- **THEN** the validation step reports the specific resource and error and fails the overall check

#### Scenario: Rendered manifest is well-formed
- **WHEN** a chart's rendered output is well-formed and matches known schemas
- **THEN** the validation step reports the resources as valid and does not fail the check for that chart

### Requirement: Custom Resource Definitions shipped by a chart are validated against their own schema
For every Custom Resource Definition the chart ships under its own `crds/` directory, the system SHALL generate a schema from that CRD and SHALL use it to validate any instance of that custom resource the chart renders, instead of merely skipping resources of that kind.

#### Scenario: Chart renders an instance of its own CRD
- **WHEN** a chart ships a CRD under `<chart>/crds/` and its rendered templates include an instance of that custom resource
- **THEN** that instance is validated against a schema generated from the chart's own CRD, and a structural error in it (for example a wrong field type or an unknown field) is reported as a validation failure, not silently skipped

#### Scenario: Chart renders an instance of a kind with no known schema
- **WHEN** a chart's rendered output includes a resource of a kind that is neither a standard Kubernetes kind nor a CRD shipped under that chart's own `crds/` directory
- **THEN** that resource is reported as skipped rather than causing the check to fail for a missing schema

### Requirement: Integration with `make lint`
The system SHALL run as part of `make lint` (alongside the existing Super-Linter and skillcheck steps) in both local (Docker) and CI invocation modes, and SHALL cause `make lint` to fail if the Helm check fails for any chart.

#### Scenario: Running `make lint` in a repository with chart issues
- **WHEN** `make lint` is run against a repository containing at least one chart with a `helm lint` or rendered-manifest validation failure
- **THEN** `make lint` exits non-zero and the failure is attributable to the Helm lint step's output
