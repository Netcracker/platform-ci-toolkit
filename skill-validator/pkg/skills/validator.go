package skills

import (
	"fmt"
	"maps"
	"slices"
)

// Severity ranks how serious a problem is. The zero value is SeverityError.
type Severity int

const (
	SeverityError   Severity = iota // The skill violates the specification
	SeverityWarning                 // Worth fixing, but does not break the skill
)

// String returns "error" or "warning".
func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	}
	return fmt.Sprintf("severity(%d)", int(s))
}

// Problem is one finding about a skill. Fields may be added in later
// versions, so build values with keyed literals.
type Problem struct {
	Severity Severity
	Details  string // Human-readable text; does not repeat the skill name
}

// String returns Details.
func (p Problem) String() string { return p.Details }

// Findings are the problems found in one skill.
type Findings struct {
	Problems []Problem // Bounded by the Validator's max problems, if set
	Skipped  bool      // The skill could not be loaded
}

// WarningLines flattens findings into one "<skill>: <text>" line per problem,
// ordered by skill name and then by discovery. A skipped skill's texts carry
// a "skipped: " prefix.
func WarningLines(findings map[string]Findings) []string {
	var out []string
	for _, name := range slices.Sorted(maps.Keys(findings)) {
		f := findings[name]
		for _, p := range f.Problems {
			w := p.Details
			if f.Skipped {
				w = "skipped: " + w
			}
			out = append(out, name+": "+w)
		}
	}
	return out
}

// WithSkipped adds a skipped entry carrying the reason for every directory
// LoadAll did not load. It returns a new map and leaves found untouched.
func WithSkipped(found map[string]Findings, skipped map[string]string) map[string]Findings {
	out := make(map[string]Findings, len(found)+len(skipped))
	for name, f := range found {
		out[name] = Findings{Problems: slices.Clone(f.Problems), Skipped: f.Skipped}
	}
	for name, reason := range skipped {
		f := out[name]
		f.Problems = append(f.Problems, Problem{Severity: SeverityError, Details: reason})
		f.Skipped = true
		out[name] = f
	}
	return out
}

// Validator checks skills against the Agent Skills specification. Create one
// with NewValidator; it is safe for concurrent use.
type Validator struct {
	maxProblems int // Per skill; 0 means unlimited
}

// Option configures a Validator.
type Option func(*Validator)

// WithMaxProblems keeps at most n problems per skill. When there are more, the
// last of the n entries counts the dropped ones ("... and N more problems").
// A value of zero or less means unlimited, which is the default.
func WithMaxProblems(n int) Option {
	return func(v *Validator) { v.maxProblems = max(n, 0) }
}

// NewValidator returns a Validator with the given options applied.
func NewValidator(opts ...Option) *Validator {
	v := &Validator{}
	for _, opt := range opts {
		opt(v)
	}
	return v
}

// ValidateAll runs Validate on every skill and returns the findings by skill
// directory name. A skill without findings has no entry.
func (v *Validator) ValidateAll(loaded []*Skill) (map[string]Findings, error) {
	out := map[string]Findings{}
	for _, s := range loaded {
		problems, err := v.Validate(s)
		if err != nil {
			return nil, fmt.Errorf("validate skill %s: %w", s.Name(), err)
		}
		if len(problems) > 0 {
			out[s.Name()] = Findings{Problems: problems}
		}
	}
	return out, nil
}

// limit bounds the problems of one skill: the list is cut to n entries, the
// last of which counts the ones dropped. A text is never cut. Zero or less
// means no bound.
func limit(problems []Problem, n int) []Problem {
	if n <= 0 {
		return problems
	}
	out := make([]Problem, 0, min(len(problems), n))
	for i, p := range problems {
		if i == n-1 && len(problems) > n {
			more := fmt.Sprintf("... and %d more problems", len(problems)-i)
			out = append(out, Problem{Severity: SeverityError, Details: more})
			break
		}
		out = append(out, p)
	}
	return out
}

// errorProblems wraps check texts as problems of severity error.
func errorProblems(texts []string) []Problem {
	out := make([]Problem, len(texts))
	for i, t := range texts {
		out[i] = Problem{Severity: SeverityError, Details: t}
	}
	return out
}

// Validate checks a loaded skill against the Agent Skills specification and
// the self-contained layout rule: every file a skill references must live
// inside the skill directory, because only that directory is installed.
//
// The returned details do not repeat the skill name. The error is reserved for
// I/O failures.
func (v *Validator) Validate(s *Skill) ([]Problem, error) {
	refs, err := checkReferences(s)
	if err != nil {
		return nil, err
	}
	out := checkFrontmatter(s)
	out = append(out, checkBodySize(s)...)
	return limit(errorProblems(append(out, refs...)), v.maxProblems), nil
}
