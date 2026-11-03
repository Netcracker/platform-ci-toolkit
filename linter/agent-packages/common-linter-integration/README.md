# common-linter-integration

APM package that teaches an AI agent how to wire the `platform-ci-toolkit` common linter into a consumer repository.

It ships one skill, `integrate-common-linter`, with a reference per CI system:

- `references/github-actions.md` for repositories hosted on GitHub (for example `qubership-dbaas`).
- `references/gitlab-ci.md` for repositories hosted on GitLab (for example `cloud-backuper`).

## Install in a consumer repository

Add the package to `apm.yml` of the consumer, then run `apm install` and `apm compile`:

```yaml
dependencies:
  apm:
    - Netcracker/platform-ci-toolkit/linter/agent-packages/common-linter-integration#<ref>
```

Pin `<ref>` to a tag once one exists. Use a branch only while a change is in review.

## Source of truth

The skill summarizes `linter/docs/usage.md` and `linter/templates/common.mk`. When those change, update the skill in the same pull request.
