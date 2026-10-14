// Command skillcheck validates Agent Skills (https://agentskills.io/specification)
// in local folders and prints the findings as a markdown report.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Netcracker/platform-ci-toolkit/skill-validator/internal/report"
	"github.com/Netcracker/platform-ci-toolkit/skill-validator/pkg/skills"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

// Exit codes.
const (
	exitOK       = 0 // No findings
	exitFindings = 1 // Findings or skipped skills
	exitError    = 2 // Invalid usage or operational error
)

const usageText = `skillcheck - validate Agent Skills before they are delivered.

USAGE
  skillcheck [flags] [folder ...]

ARGUMENTS
  folder   A local folder to verify; may be repeated. A folder that contains
           a SKILL.md is validated as one skill. Any other folder is a bundle
           root: each of its immediate subdirectories that contains a SKILL.md
           is validated as a skill; other subdirectories are ignored. It is an
           error when a folder holds no skills.
           Without arguments the current directory is used, as if "." was
           passed.

FLAGS
  -h, --help      Print this help and exit.
  --version       Print the version and exit.
  --max-problems N
                  Report at most N problems per skill; the last line counts
                  the ones left out. 0 means no limit, which is the default.
  --no-color      Do not color the report. Colors are also off when the
                  NO_COLOR environment variable is set to a non-empty value.

CHECKS
  name           Required; at most 64 characters; lowercase letters, digits and
                 hyphens only; no leading, trailing or consecutive hyphens;
                 equal to the skill directory name.
  description    Required; not blank; at most 1024 characters.
  optional       license, compatibility and allowed-tools must be strings;
  fields         compatibility has at most 500 characters; metadata must map
                 strings to strings; other frontmatter keys are reported.
  body size      The SKILL.md body has at most 500 lines.
  code fences    Every fenced code block in a Markdown file must be closed.
  layout         Every file a skill references must live inside the skill
                 folder: Markdown links must resolve there, and references
                 that point outside it are reported.
  loading        The skill file must be named exactly SKILL.md; a name that
                 differs in case, such as skill.md, is reported. A SKILL.md
                 with unparsable frontmatter is reported as skipped.

OUTPUT
  A plain-text report on standard output. Every validated skill gets a line:
    PASS  <skill>   no check failed
    FAIL  <skill>   at least one check failed; the problems follow, one per line
    WARN  <skill>   the skill could not be loaded and was skipped; the reason
                    follows
  The last line is a summary: skills checked, valid, failed, skipped, and the
  number of problems. PASS is green, FAIL red and WARN yellow, also when the
  output is not a terminal (see --no-color). Errors and usage problems go to
  standard error.

EXIT CODES
  0   Every skill passed.
  1   At least one skill failed or was skipped.
  2   Invalid usage or an operational error (for example, a folder that does
      not exist or holds no skills).
`

// fail prints an error line to w.
func fail(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, "Error: "+format+"\n", args...)
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes skillcheck with args (without the program name) and returns the
// process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("skillcheck", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	showVersion := fs.Bool("version", false, "")
	noColor := fs.Bool("no-color", false, "")
	maxProblems := fs.Int("max-problems", 0, "")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usageText)
			return exitOK
		}
		fmt.Fprintf(stderr, "Error: %v\n\n%s", err, usageText)
		return exitError
	}
	if *showVersion {
		fmt.Fprintf(stdout, "skillcheck %s\n", version)
		return exitOK
	}

	if *maxProblems < 0 {
		fmt.Fprintf(stderr, "Error: --max-problems must not be negative\n\n%s", usageText)
		return exitError
	}
	v := skills.NewValidator(skills.WithMaxProblems(*maxProblems))

	folders := fs.Args()
	if len(folders) == 0 {
		folders = []string{"."}
	}
	targets, ok := resolve(folders, stderr)
	if !ok {
		return exitError
	}

	var res report.Result
	failed := false
	for _, t := range targets {
		if err := collect(t, v, &res); err != nil {
			fail(stderr, "%v", err)
			failed = true
		}
	}
	if failed {
		return exitError
	}
	color := !*noColor && os.Getenv("NO_COLOR") == ""
	if err := report.Render(stdout, res, color); err != nil {
		fail(stderr, "write report: %v", err)
		return exitError
	}
	if res.Failed() {
		return exitFindings
	}
	return exitOK
}

// target is one folder argument: as the user wrote it, and as an absolute path.
type target struct{ arg, dir string }

// resolve turns the arguments into distinct absolute directories. Every
// problem is reported before it gives up, so one run shows all bad arguments.
func resolve(folders []string, stderr io.Writer) ([]target, bool) {
	var targets []target
	seen := map[string]bool{}
	ok := true
	for _, f := range folders {
		abs, err := filepath.Abs(f)
		if err != nil {
			fail(stderr, "%s: %v", f, err)
			ok = false
			continue
		}
		info, err := os.Stat(abs)
		switch {
		case err != nil:
			fail(stderr, "%s: %v", f, errors.Unwrap(err))
			ok = false
		case !info.IsDir():
			fail(stderr, "%s: not a directory", f)
			ok = false
		case !seen[abs]:
			seen[abs] = true
			targets = append(targets, target{arg: f, dir: abs})
		}
	}
	return targets, ok
}

// collect validates the skill or bundle at t and adds the outcome to res. It
// returns an error when t holds no skill.
func collect(t target, v *skills.Validator, res *report.Result) error {
	var (
		loaded  []*skills.Skill
		skipped map[string]string
	)
	name, _, err := skills.FindSkillFile(t.dir)
	if err != nil {
		return fmt.Errorf("%s: %w", t.arg, err)
	}
	single := name != ""
	switch {
	case single:
		// t is one skill. A skill file that cannot be loaded (bad frontmatter,
		// wrong case) is a skipped skill, like the ones found inside a bundle.
		s, err := skills.Load(t.dir)
		if err != nil {
			skipped = map[string]string{filepath.Base(t.dir): err.Error()}
		} else {
			loaded = []*skills.Skill{s}
		}
	default:
		if loaded, skipped, err = skills.LoadAll(t.dir); err != nil {
			return err
		}
		if len(loaded)+len(skipped) == 0 {
			return fmt.Errorf("no skills found in %s (expected %s or subdirectories containing %s)", t.arg, skills.SkillFile, skills.SkillFile)
		}
	}

	found, err := v.ValidateAll(loaded)
	if err != nil {
		return err
	}
	all := skills.WithSkipped(found, skipped)

	entry := func(name string, f skills.Findings) report.Entry {
		dir := t.dir
		if !single {
			dir = filepath.Join(t.dir, name)
		}
		return report.Entry{Name: name, Dir: dir, Findings: f}
	}
	for _, s := range loaded {
		res.Entries = append(res.Entries, entry(s.Name(), all[s.Name()]))
	}
	for name := range skipped {
		res.Entries = append(res.Entries, entry(name, all[name]))
	}
	return nil
}
