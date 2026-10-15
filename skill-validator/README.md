# skillcheck

`skillcheck` validates [Agent Skills](https://agentskills.io/specification) in local folders and prints a colored text report. Run it while you develop a skill, or in CI to fail a merge when a skill has defects, before the skill bundle is delivered to an AI agent.

The executable is self-contained: no Go toolchain, build system or source checkout is needed to run it.

## Usage

```
skillcheck [flags] [folder ...]
```

| Argument | Meaning |
|----------|---------|
| `folder` | A local folder to verify. May be repeated. A folder that contains a `SKILL.md` is validated as one skill. Any other folder is a bundle root: each of its immediate subdirectories that contains a `SKILL.md` is validated as a skill; other subdirectories are ignored. |

Without arguments the current directory is used, as if `.` was passed: it is validated as one skill if it contains a `SKILL.md`, otherwise its subdirectories with a `SKILL.md` are validated.

It is an error (exit code 2) when a folder holds no skills:

```
Error: no skills found in ./src (expected SKILL.md or subdirectories containing SKILL.md)
```

The skill file must be named exactly `SKILL.md`. A file such as `skill.md` is reported, even on case-insensitive file systems (Windows, macOS) where it would otherwise load.

| Flag | Meaning |
|------|---------|
| `-h`, `--help` | Print usage, arguments, checks, output format and exit codes. |
| `--version` | Print the version. |
| `--max-problems N` | Report at most `N` problems per skill; the last line counts the ones left out. `0` means no limit and is the default. A negative value is a usage error. |
| `--no-color` | Do not color the report. Same as setting the `NO_COLOR` environment variable to a non-empty value. |

Examples:

```sh
skillcheck                          # the current directory, same as `skillcheck .`
skillcheck ./skills                 # a bundle of skills
skillcheck ./skills/code-review     # one skill
skillcheck ./team-a/skills ./team-b/skills
```

## Output

The report goes to standard output. Every validated skill gets one line, sorted by name:

| Label | Meaning |
|-------|---------|
| `PASS` (green) | No check failed. |
| `FAIL` (red) | At least one check failed. Each problem is listed below, one per line. |
| `WARN` (yellow) | The skill could not be loaded and was skipped (unparsable frontmatter, or a wrongly cased skill file). The reason is listed below. |

The last line is the summary: skills checked, valid, failed, skipped, and the number of problems. When two listed skills share a name, the folder is shown after the name.

```
FAIL  Bad_Skill
        SKILL.md: name "other" must equal the directory name "Bad_Skill"
        SKILL.md: description is missing or empty
        SKILL.md:4: "../shared.md" points outside the skill folder; only .apm/skills/Bad_Skill/ is installed
PASS  code-review
WARN  torn
        parse frontmatter SKILL.md: yaml: line 1: did not find expected ',' or ']'

Summary: 3 skills checked, 1 valid, 1 failed, 1 skipped, 3 problems
```

Colors are always on, also when the output is piped, so that CI logs show them. Set `NO_COLOR=1` or pass `--no-color` to get plain text. Errors and usage problems go to standard error.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Every skill passed. |
| 1 | At least one skill failed or was skipped. |
| 2 | Invalid usage or an operational error, for example a folder that does not exist or holds no skills. |

## Checks

Every skill is checked against the Agent Skills specification and one layout rule.

Each item is one thing the tool verifies. Replace `x`, `y`, `k`, `N` and `<...>` with the values from your report.

**Loading** (reported as `WARN`; the skill is skipped and not validated further)

- [ ] The skill file is named exactly `SKILL.md`
  - Error: `found "skill.md", but the file must be named SKILL.md`
  - Fix: rename the file to `SKILL.md`.
- [ ] The frontmatter is valid YAML and its fields have the right shape
  - Error: `parse frontmatter SKILL.md: yaml: ...`
  - Fix: correct the YAML between the `---` lines. This includes a list where a string is expected (`license: [a, b]`) and a string where a map is expected (`metadata: text`).

Subdirectories of a bundle root without a skill file are not skills and are ignored.

**`name`**

- [ ] `name` is present
  - Error: `SKILL.md: name is missing`
  - Fix: add `name:` to the frontmatter.
- [ ] `name` has at most 64 characters
  - Error: `SKILL.md: name "x" is longer than 64 characters`
  - Fix: shorten the name.
- [ ] `name` uses only lowercase letters, digits and hyphens
  - Error: `SKILL.md: name "x" may contain only lowercase letters, digits and hyphens`
  - Fix: remove uppercase letters, underscores, spaces and other characters.
- [ ] `name` does not start or end with a hyphen
  - Error: `SKILL.md: name "x" must not start or end with a hyphen`
  - Fix: remove the leading or trailing `-`.
- [ ] `name` has no consecutive hyphens
  - Error: `SKILL.md: name "x" must not contain consecutive hyphens`
  - Fix: replace `--` with a single `-`.
- [ ] `name` equals the skill directory name
  - Error: `SKILL.md: name "x" must equal the directory name "y"`
  - Fix: rename the folder or change `name` so that both are the same.

**`description`**

- [ ] `description` is present and not blank
  - Error: `SKILL.md: description is missing or empty`
  - Fix: add a non-blank `description:`.
- [ ] `description` has at most 1024 characters
  - Error: `SKILL.md: description is longer than 1024 characters`
  - Fix: shorten it; move details into the body or into `references/`.

**Optional frontmatter fields**

- [ ] `license` and `allowed-tools` are strings
  - Error: `SKILL.md: license must be a string` (same for `allowed-tools`)
  - Fix: write the value as a string; quote it if YAML reads it as a number or boolean.
- [ ] `compatibility` is a string of at most 500 characters
  - Error: `SKILL.md: compatibility must be a string` or `SKILL.md: compatibility is longer than 500 characters`
  - Fix: use a string and shorten it.
- [ ] `metadata` maps strings to strings
  - Error: `SKILL.md: metadata value of "k" must be a string` or `SKILL.md: metadata key K must be a string`
  - Fix: quote the value or key (`version: "1"`).
- [ ] The frontmatter has no unknown keys (allowed: `name`, `description`, `license`, `compatibility`, `metadata`, `allowed-tools`)
  - Error: `SKILL.md: unknown frontmatter field "x"`
  - Fix: remove the key, fix a typo in a known key, or move the data under `metadata:`.

**`SKILL.md` body and Markdown files**

- [ ] The `SKILL.md` body, the part after the frontmatter, has at most 500 lines
  - Error: `SKILL.md: body has N lines, more than 500`
  - Fix: move detail into files under `references/` and link them from `SKILL.md`.
- [ ] Every fenced code block in every Markdown file is closed
  - Error: `<file>:<line>: code fence is not closed` (`<line>` is the opening fence)
  - Fix: add a closing fence of the same character (`` ` `` or `~`) that is at least as long as the opening one.

**Self-contained layout** (everything a skill references must be inside the skill folder)

- [ ] Links, images, link definitions and code spans stay inside the skill folder
  - Error: `<file>:<line>: "ref" points outside the skill folder; only .apm/skills/<name>/ is installed`
  - Fix: copy the file into the skill folder and link it with a relative path, or remove the reference. Targets starting with `../` or `/`, or containing `.apm/`, are reported; fenced code blocks, links written inside inline code spans (samples and templates) and links with a scheme (`https:`, `mailto:`) are ignored.
- [ ] Linked files exist
  - Error: `<file>:<line>: "ref" does not exist in the skill folder`
  - Fix: create the file or correct the link path.

All problems of a skill are reported by default. With `--max-problems N` at most `N` are listed per skill and the last line tells how many were left out.

Not checked yet: the size of the `SKILL.md` body in tokens (only lines are counted). Semantic checks of skill content are out of scope.

## Use in CI

The exit code makes `skillcheck` a merge gate. For example, in a GitHub Actions job:

```yaml
- name: Validate skills
  run: ./skillcheck ./skills
```

Any job runner works the same way: the step fails when the exit code is not 0. Most CI log viewers render the colors; add `--no-color` if yours does not.

## Install and use

### As a binary

**Local development: `go install`**

Requires Go (version in `go.mod`). The executable lands in `$(go env GOPATH)/bin`; make sure that folder is on your `PATH`.

```sh
go install github.com/Netcracker/platform-ci-toolkit/skill-validator/cmd/skillcheck@latest
skillcheck ./skills
```

`skill-validator` is a subdirectory module, so Go resolves versions from tags named `skill-validator/vX.Y.Z`. Without such a tag `@latest` resolves to a pseudo-version of the default branch; use `@main` to track it explicitly.

**CI: Docker image**

The image has no Go toolchain, only the static executable. The entrypoint is `skillcheck` and the working directory is `/work`, so mount the repository there and pass folders as arguments:

```sh
docker run --rm -v "$PWD:/work" ghcr.io/netcracker/skillcheck:latest ./skills
```

**Clone and build**

Requires Go and `make`.

```sh
git clone https://github.com/Netcracker/platform-ci-toolkit.git
cd platform-ci-toolkit/skill-validator
make build            # -> dist/skillcheck
./dist/skillcheck ./skills
```

See [Build](#build) for all make targets.

### As a Go library

The checks are available as the package `pkg/skills`, the same code the command uses. Reports are not part of the library: you get the problems as plain strings, identical to the command's messages, and decide how to present them.

```sh
go get github.com/Netcracker/platform-ci-toolkit/skill-validator@latest
```

```go
package main

import (
	"fmt"
	"log"

	"github.com/Netcracker/platform-ci-toolkit/skill-validator/pkg/skills"
)

func main() {
	s, err := skills.Load("./skills/code-review")
	if err != nil {
		log.Fatal(err) // not loadable: bad frontmatter or wrongly cased skill file
	}
	problems, err := skills.NewValidator().Validate(s)
	if err != nil {
		log.Fatal(err)
	}
	for _, p := range problems {
		fmt.Printf("%s: [%s] %s\n", s.Name(), p.Severity, p.Details)
	}
}
```

`Validate` returns `[]skills.Problem`. A `Problem` has a `Severity` (`SeverityError` or `SeverityWarning`) and `Details`, the message text. Today every problem is an error and the command fails on any problem. Build `Problem` values with keyed literals: fields may be added later.

To check every skill in a bundle folder, use `skills.LoadAll(root)` (returns loaded skills and the skipped ones with reasons), then `v.ValidateAll(loaded)` and `skills.WarningLines(...)`, where `v` is the validator from `skills.NewValidator()`. Pass options to configure it: `skills.NewValidator(skills.WithMaxProblems(20))` keeps at most 20 problems per skill (the default is no limit).

## Build

Requires Go (version in `go.mod`) and `make`.

```sh
make test        # run the tests
make lint        # go vet and gofmt check
make build       # host platform -> dist/skillcheck
make build-all   # linux, darwin, windows x amd64, arm64 -> dist/
make build-all VERSION=1.2.3   # set the version printed by --version
```

Executables are static (`CGO_ENABLED=0`). Publishing release artifacts is not part of this tool yet.
