# skillcheck

CLI that validates [Agent Skills](https://agentskills.io/specification) in local folders and prints a colored text report. Used locally by skill authors and in CI to gate merges. Usage, checks and exit codes are documented in `README.md`; keep it in sync with behavior.

## Commands

Run from `skill-validator/`:

```sh
make test                              # go test ./...
make lint                              # go vet + gofmt check (fails if files need gofmt)
make build                             # host binary -> dist/skillcheck(.exe)
make build-all VERSION=1.2.3           # cross-compiled static binaries -> dist/
go test ./internal/report -update      # rewrite golden files after an intended report change
```

`dist/` is git-ignored. The Go version is pinned in `go.mod` (module path `github.com/Netcracker/platform-ci-toolkit/skill-validator`). Dependencies: stdlib plus `github.com/adrg/frontmatter`; avoid adding more.

## Layout

- `cmd/skillcheck/main.go`: flags, folder handling, exit codes, help text. `run(args, stdout, stderr) int` is the testable entry point; `main` only calls it.
- `pkg/skills`: public library API (importable by Go users; exported names are a compatibility surface: configure `Validator` through `Option`s passed to `NewValidator`, never new constructor parameters; add `Problem` fields, never reorder or remove, callers use keyed literals). Loading (`Load`, `LoadAll`, `FindSkillFile`) and validation (`Validate`, `ValidateAll`). Ported from an internal library; checks and message wording are meant to stay identical.
- `internal/report`: text rendering with ANSI colors. Golden files live in `internal/report/testdata/`.

## Rules that are not obvious from the code

- **Warnings:** keys in `harnessFields` (Claude Code extensions) are `SeverityWarning`; they print yellow, leave the skill `PASS` and the exit code 0. Other unknown keys stay errors. `--strict` / `WithStrict()` turns every warning into an error. Only add to `harnessFields` keys that a named harness documents.
- **Exit codes:** 0 all skills pass (warnings do not count), 1 any skill failed or was skipped, 2 usage or operational error (missing folder, folder without skills, bad flag). Changing them means updating the help text and README too.
- **The skill file is matched by exact case.** Always find it through `skills.FindSkillFile` (directory listing); `os.Stat("SKILL.md")` also matches `skill.md` on Windows and macOS.
- **Folder semantics:** a folder with `SKILL.md` is one skill; otherwise it is a bundle root and only subdirectories with a skill file count. With no arguments the current directory is used, exactly as if `.` was passed.
- **Finding messages are a compatibility surface:** CI users read them. Change wording only deliberately, with tests.
- **Colors are always on**, also when piped. Opt out only via `NO_COLOR` (non-empty) or `--no-color`. No TTY detection, and no OS-specific code or build tags: behavior must be identical on every platform.
- **Output streams:** the report goes to stdout; `Error: ...` lines and usage problems go to stderr.
- Tests use `t.Setenv("NO_COLOR", ...)`. CLI tests call `run` in-process; `TestBinary` builds the real binary.
