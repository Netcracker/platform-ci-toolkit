package report

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Netcracker/platform-ci-toolkit/skill-validator/pkg/skills"
)

var update = flag.Bool("update", false, "rewrite golden files")

func problems(ws ...string) skills.Findings { return skills.Findings{Problems: wrap(ws)} }

func wrap(ws []string) []skills.Problem {
	var out []skills.Problem
	for _, w := range ws {
		out = append(out, skills.Problem{Details: w})
	}
	return out
}

func skipped(w string) skills.Findings {
	return skills.Findings{Problems: wrap([]string{w}), Skipped: true}
}

var (
	allValid = Result{Entries: []Entry{{Name: "zeta"}, {Name: "alpha"}}}

	mixed = Result{Entries: []Entry{
		{Name: "torn", Findings: skipped("parse frontmatter SKILL.md: yaml: line 1: did not find expected ','")},
		{Name: "code-review"},
		{Name: "bad", Findings: problems("SKILL.md: name is missing", `SKILL.md:5: "../x.md" points outside the skill folder`)},
	}}

	onlySkipped = Result{Entries: []Entry{{Name: "torn", Findings: skipped(`found "skill.md", but the file must be named SKILL.md`)}}}

	sameName = Result{Entries: []Entry{
		{Name: "dup", Dir: "/work/two"},
		{Name: "dup", Dir: "/work/one", Findings: problems("SKILL.md: description is missing or empty")},
		{Name: "solo", Dir: "/work/one"},
	}}

	warnOnly = Result{Entries: []Entry{
		{Name: "ok"},
		{Name: "portable", Findings: skills.Findings{Problems: []skills.Problem{
			{Severity: skills.SeverityWarning, Details: `SKILL.md: harness-specific frontmatter field "model" (Claude Code only, not in the Agent Skills specification)`},
		}}},
	}}

	warnAndError = Result{Entries: []Entry{
		{Name: "both", Findings: skills.Findings{Problems: []skills.Problem{
			{Severity: skills.SeverityError, Details: "SKILL.md: name is missing"},
			{Severity: skills.SeverityWarning, Details: `SKILL.md: harness-specific frontmatter field "paths" (Claude Code only, not in the Agent Skills specification)`},
		}}},
	}}

	multiline = Result{Entries: []Entry{
		{Name: "md", Findings: problems("a | b `code` [link](x) *bold*\nsecond\tline   spaced")},
	}}
)

func TestRenderGolden(t *testing.T) {
	tests := []struct {
		name  string
		res   Result
		color bool
	}{
		{"empty", Result{}, false},
		{"all_valid", allValid, false},
		{"mixed", mixed, false},
		{"only_skipped", onlySkipped, false},
		{"same_name", sameName, false},
		{"multiline", multiline, false},
		{"warnings_only", warnOnly, false},
		{"warnings_only_color", warnOnly, true},
		{"warnings_and_errors", warnAndError, false},
		{"mixed_color", mixed, true},
		{"all_valid_color", allValid, true},
		{"only_skipped_color", onlySkipped, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Render(&buf, tc.res, tc.color); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("testdata", tc.name+".golden")
			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.ReplaceAll(buf.String(), "\r\n", "\n"); got != strings.ReplaceAll(string(want), "\r\n", "\n") {
				t.Errorf("output differs from %s\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
			}
		})
	}
}

func TestNoEscapeSequencesWithoutColor(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, mixed, false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "\x1b") {
		t.Errorf("escape sequence in uncolored output:\n%q", buf.String())
	}
}

func TestColors(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, mixed, true); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"\x1b[32mPASS\x1b[0m  code-review",
		"\x1b[31mFAIL\x1b[0m  bad",
		"\x1b[33mWARN\x1b[0m  torn",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%q", want, out)
		}
	}
	// The summary takes the worst color: red when something failed.
	last := out[strings.LastIndex(strings.TrimSuffix(out, "\n"), "\n")+1:]
	if !strings.HasPrefix(last, "\x1b[1m\x1b[31mSummary:") {
		t.Errorf("summary line = %q", last)
	}
}

func TestSummaryIsLast(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, mixed, false); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if got := lines[len(lines)-1]; got != "Summary: 3 skills checked, 1 valid, 1 failed, 1 skipped, 3 problems" {
		t.Errorf("last line = %q", got)
	}
	if strings.Count(buf.String(), "Summary:") != 1 {
		t.Error("summary must appear once")
	}
}

func TestStatusAndFailed(t *testing.T) {
	if (Entry{}).Status() != Pass || (Entry{Findings: problems("x")}).Status() != Fail || (Entry{Findings: skipped("x")}).Status() != Warn {
		t.Error("wrong status")
	}
	if allValid.Failed() || !mixed.Failed() || !onlySkipped.Failed() || (Result{}).Failed() {
		t.Error("wrong Failed()")
	}
	if warnOnly.Failed() || warnOnly.Entries[1].Status() != Pass {
		t.Error("warnings alone must not fail")
	}
	if !warnAndError.Failed() || warnAndError.Entries[0].Status() != Fail {
		t.Error("an error next to a warning must fail")
	}
}

func TestWarningColors(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, warnOnly, true); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		yellow + "        SKILL.md: harness-specific",
		bold + yellow + "Summary:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%q", want, out)
		}
	}
}

func TestRenderDoesNotMutateInput(t *testing.T) {
	r := Result{Entries: []Entry{{Name: "b"}, {Name: "a"}}}
	if err := Render(&bytes.Buffer{}, r, false); err != nil {
		t.Fatal(err)
	}
	if r.Entries[0].Name != "b" {
		t.Error("Render reordered the caller's entries")
	}
}
