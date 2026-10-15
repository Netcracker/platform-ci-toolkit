# Platform CI Toolkit

This repository contains shared CI/CD utilities used across Netcracker projects.

## Contents

| Directory | Purpose |
| --- | --- |
| `linter/` | Common containerized linter based on GitHub Super-Linter. See `linter/README.md` for build and usage instructions. |
| `skill-validator/` | Skill validation utility for APM packages. |

## Quick start

```bash
# Build the common linter image
make build-linter

# Run the common linter against this repository
make lint-linter
```
