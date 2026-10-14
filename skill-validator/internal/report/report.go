// Package report renders skill validation results as a plain-text report for
// terminals and CI logs.
package report

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"strings"

	"skillcheck/internal/skills"
)

// Status is the outcome of one skill. The order is the severity order.
type Status int

const (
	Pass Status = iota // No check failed
	Warn               // The skill could not be loaded and was skipped
	Fail               // At least one check failed
)

// Entry is one validated skill with its findings, if any.
type Entry struct {
	Name     string // Skill directory name
	Dir      string // Folder of the skill, shown when names clash
	Findings skills.Findings
}

// Status derives the outcome from the findings.
func (e Entry) Status() Status {
	switch {
	case e.Findings.Skipped:
		return Warn
	case len(e.Findings.Warnings) > 0:
		return Fail
	}
	return Pass
}

// Result is everything one run checked: every skill, valid or not.
type Result struct {
	Entries []Entry
}

// Failed reports whether any skill failed or was skipped.
func (r Result) Failed() bool {
	return slices.ContainsFunc(r.Entries, func(e Entry) bool { return e.Status() != Pass })
}

const (
	reset  = "\x1b[0m"
	bold   = "\x1b[1m"
	green  = "\x1b[32m"
	yellow = "\x1b[33m"
	red    = "\x1b[31m"
)

// label and colorOf give the text and color of a status.
var (
	label   = map[Status]string{Pass: "PASS", Warn: "WARN", Fail: "FAIL"}
	colorOf = map[Status]string{Pass: green, Warn: yellow, Fail: red}
)

// Render writes r as text, with ANSI colors when color is true. Output is
// deterministic: skills are ordered by name and then by folder, problems keep
// their discovery order. The summary is the last thing written.
func Render(w io.Writer, r Result, color bool) error {
	paint := func(code, s string) string {
		if !color {
			return s
		}
		return code + s + reset
	}

	entries := slices.Clone(r.Entries)
	slices.SortStableFunc(entries, func(a, b Entry) int {
		return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.Dir, b.Dir))
	})
	names := map[string]int{}
	for _, e := range entries {
		names[e.Name]++
	}

	var b strings.Builder
	var count [3]int
	problems := 0
	for _, e := range entries {
		st := e.Status()
		count[st]++
		problems += len(e.Findings.Warnings)

		b.WriteString(paint(colorOf[st], label[st]) + "  " + e.Name)
		if names[e.Name] > 1 && e.Dir != "" {
			b.WriteString(" (" + e.Dir + ")")
		}
		b.WriteString("\n")
		for _, f := range e.Findings.Warnings {
			b.WriteString("        " + oneLine(f) + "\n")
		}
	}

	worst := Pass
	for st := range count {
		if count[st] > 0 {
			worst = max(worst, Status(st))
		}
	}
	summary := fmt.Sprintf("Summary: %s checked, %d valid, %d failed, %d skipped, %s",
		plural(len(entries), "skill"), count[Pass], count[Fail], count[Warn], plural(problems, "problem"))
	if len(entries) > 0 {
		b.WriteString("\n")
	}
	b.WriteString(paint(bold+colorOf[worst], summary) + "\n")

	_, err := io.WriteString(w, b.String())
	return err
}

// oneLine collapses all white space, line breaks included, so that every
// problem stays on its own line.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}
