# GitHub Actions integration

Add `.github/workflows/lint.yaml`. This is the shape used by `qubership-dbaas`; keep action pins consistent with the
repository's other workflows (commit SHA with a version comment if that is the local convention).

```yaml
name: Lint

on:
  pull_request:
    branches: [main]
  push:
    branches: [main]

permissions:
  contents: read
  packages: read

jobs:
  lint:
    runs-on: ubuntu-latest
    container:
      image: ghcr.io/netcracker/linter:latest
      options: --entrypoint=""
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Configure Git authentication for private Go modules
        run: |
          git config --global url."https://token:${{ secrets.GITHUB_TOKEN }}@github.com/".insteadOf "https://github.com/"

      - name: Run lint
        run: make lint
        env:
          RUN_LOCAL: "true"
```

Replace `main` with the repository default branch. The image tag must match `LINTER_TAG` in the wrapper `Makefile`.

## Tests job

Add a `tests` job only when the repository has Go, Python, or Java tests. Use a matrix with one entry per language
(`target`: `test-go`, `test-python`, `test-java`; `image`: `golang:<go.mod version>`, `python:<version>`,
`maven:3-eclipse-temurin-<jdk>`). Do not run tests in the linter image. Each job runs `make <target>` in its own
image, which must provide `make` and `git` (the standard images do; see `docs/usage.md` for the full example).
Add only the matrix entries, cache steps and env that apply:

- Go: cache `.go/pkg/mod` keyed on `hashFiles('**/go.sum')` and set `GOMODCACHE` to
  `${{ github.workspace }}/.go/pkg/mod`.
- Python: cache `.pip-cache` keyed on `hashFiles('**/requirements*.txt')` and set `PIP_CACHE_DIR` to
  `${{ github.workspace }}/.pip-cache`.
- Maven: cache `.m2/repository` keyed on `hashFiles('**/pom.xml')` and set `MAVEN_OPTS` to
  `-Dmaven.repo.local=${{ github.workspace }}/.m2/repository`.
- Maven dependencies from private GitHub Packages: write `~/.m2/settings.xml` with a `<server>` whose `<id>` equals
  the repository id in `pom.xml`, using `github.actor` and `secrets.GITHUB_TOKEN`.
- `GOPROXY: ${{ vars.GOPROXY }}` when the public proxy is unreachable or a private proxy is required.

The linter image has no Maven or Java; the Maven image supplies both.

## Replacing existing jobs

If the repository already has per-language lint or test workflows (go vet, golangci-lint, markdownlint, yamllint,
`mvn test`), list them to the user and ask before deleting. Super-Linter covers most linters; unit-test workflows
that need services or secrets may need to stay.

## Branch protection

After the first green run, tell the user to add the `lint` (and `tests`) check as a required status check. That is a
repository setting; do not attempt it.
