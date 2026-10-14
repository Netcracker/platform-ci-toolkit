package skills

import (
	"fmt"
	"maps"
	"slices"
)

const maxWarnings = 20 // Warnings kept per skill

// Findings are the problems found in one skill.
type Findings struct {
	Warnings []string // One text per problem, at most maxWarnings
	Skipped  bool     // The skill could not be loaded
}

// WarningLines flattens findings into one "<skill>: <text>" line per warning,
// ordered by skill name and then by discovery. A skipped skill's texts carry
// a "skipped: " prefix.
func WarningLines(findings map[string]Findings) []string {
	var out []string
	for _, name := range slices.Sorted(maps.Keys(findings)) {
		f := findings[name]
		for _, w := range f.Warnings {
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
		out[name] = Findings{Warnings: slices.Clone(f.Warnings), Skipped: f.Skipped}
	}
	for name, reason := range skipped {
		f := out[name]
		f.Warnings = limit(append(f.Warnings, reason))
		f.Skipped = true
		out[name] = f
	}
	return out
}

// ValidateAll runs Validate on every skill and returns the findings by skill
// directory name. A skill without findings has no entry.
func ValidateAll(loaded []*Skill) (map[string]Findings, error) {
	out := map[string]Findings{}
	for _, s := range loaded {
		warns, err := Validate(s)
		if err != nil {
			return nil, fmt.Errorf("validate skill %s: %w", s.Name(), err)
		}
		if len(warns) > 0 {
			out[s.Name()] = Findings{Warnings: warns}
		}
	}
	return out, nil
}

// limit bounds the warnings of one skill: the list is cut to maxWarnings
// entries, the last of which counts the ones dropped. A text is never cut.
func limit(warns []string) []string {
	out := make([]string, 0, min(len(warns), maxWarnings))
	for i, w := range warns {
		if i == maxWarnings-1 && len(warns) > maxWarnings {
			out = append(out, fmt.Sprintf("... and %d more problems", len(warns)-i))
			break
		}
		out = append(out, w)
	}
	return out
}

// Validate checks a loaded skill against the Agent Skills specification and
// the self-contained layout rule: every file a skill references must live
// inside the skill directory, because only that directory is installed.
//
// The returned texts do not repeat the skill name. The error is reserved for
// I/O failures.
func Validate(s *Skill) ([]string, error) {
	refs, err := checkReferences(s)
	if err != nil {
		return nil, err
	}
	out := checkFrontmatter(s)
	out = append(out, checkBodySize(s)...)
	return limit(append(out, refs...)), nil
}
