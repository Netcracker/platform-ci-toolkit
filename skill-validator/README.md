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

Without arguments the current directory is validated as **one skill**; its subdirectories are not searched. To validate a bundle, pass its folder.

It is an error (exit code 2) when a folder holds no skills:

```
Error: no skills found in ./src (expected SKILL.md or subdirectories containing SKILL.md)
```

The skill file must be named exactly `SKILL.md`. A file such as `skill.md` is reported, even on case-insensitive file systems (Windows, macOS) where it would otherwise load.

| Flag | Meaning |
|------|---------|
| `-h`, `--help` | Print usage, arguments, checks, output format and exit codes. |
| `--version` | Print the version. |
| `--no-color` | Do not color the report. Same as setting the `NO_COLOR` environment variable to a non-empty value. |

Examples:

```sh
skillcheck                          # the current directory is one skill
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

| Check | Rule |
|-------|------|
| `name` | Required. At most 64 characters. Lowercase letters, digits and hyphens only. Must not start or end with a hyphen, or contain consecutive hyphens. Must equal the skill directory name. |
| `description` | Required, not blank, at most 1024 characters. |
| Self-contained layout | Every file a skill references must live inside the skill folder. In every Markdown file of the skill, links, images and link definitions that point outside the folder (`../`, absolute paths, `.apm/`) are reported, as are links whose target does not exist inside the folder. Code spans that name a file outside the skill (starting with `../` or containing `.apm/`) are reported too. Fenced code blocks and links with a scheme (`https:`, `mailto:`) are ignored. |
| Loading | The skill file must be named exactly `SKILL.md`; a differently cased name is reported. A `SKILL.md` whose frontmatter cannot be parsed is reported as skipped. Subdirectories of a bundle root without any skill file are not skills and are ignored. |

At most 20 problems are reported per skill; when there are more, the last line tells how many were left out.

Not checked yet: `license`, `compatibility`, `metadata`, `allowed-tools` and the size of the `SKILL.md` body. Semantic checks of skill content are out of scope.

## Use in CI

The exit code makes `skillcheck` a merge gate. For example, in a GitHub Actions job:

```yaml
- name: Validate skills
  run: ./skillcheck ./skills
```

Any job runner works the same way: the step fails when the exit code is not 0. Most CI log viewers render the colors; add `--no-color` if yours does not.

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
