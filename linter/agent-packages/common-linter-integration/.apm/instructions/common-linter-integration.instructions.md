---
description: Trigger for integrating the common linter into a consumer repository.
applyTo: "**/{Makefile,Makefile.precheck,.gitlab-ci.yml,lint.yml,lint.yaml}"
---

When adding or changing `make lint` / `make test` wiring, `Makefile.precheck`, or a lint job in
`.gitlab-ci.yml` or `.github/workflows/`, apply the `integrate-common-linter` skill.
