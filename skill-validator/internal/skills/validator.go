package skills

import (
	"bufio"
	"fmt"
	"io/fs"
	"maps"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	maxNameLen        = 64
	maxDescriptionLen = 1024

	maxWarnings = 20 // Warnings kept per skill
)

var (
	inlineLinkRe = regexp.MustCompile("!?\\[[^\\]]*\\]\\(\\s*<?([^)\\s>]+)>?(?:\\s+\"[^\"]*\")?\\s*\\)")
	refDefRe     = regexp.MustCompile(`^\s{0,3}\[[^\]]+\]:\s*<?([^\s>]+)>?`)
	codeSpanRe   = regexp.MustCompile("`([^`\\n]+)`")
	schemeRe     = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)
)

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
	return limit(append(checkFrontmatter(s), refs...)), nil
}

func checkFrontmatter(s *Skill) []string {
	var out []string

	name := s.Meta.Name
	switch {
	case name == "":
		out = append(out, "SKILL.md: name is missing")
	default:
		if utf8.RuneCountInString(name) > maxNameLen {
			out = append(out, fmt.Sprintf("SKILL.md: name %q is longer than %d characters", name, maxNameLen))
		}
		if strings.IndexFunc(name, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-')
		}) >= 0 {
			out = append(out, fmt.Sprintf("SKILL.md: name %q may contain only lowercase letters, digits and hyphens", name))
		}
		if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
			out = append(out, fmt.Sprintf("SKILL.md: name %q must not start or end with a hyphen", name))
		}
		if strings.Contains(name, "--") {
			out = append(out, fmt.Sprintf("SKILL.md: name %q must not contain consecutive hyphens", name))
		}
		if name != s.Name() {
			out = append(out, fmt.Sprintf("SKILL.md: name %q must equal the directory name %q", name, s.Name()))
		}
	}

	desc := strings.TrimSpace(s.Meta.Description)
	switch {
	case desc == "":
		out = append(out, "SKILL.md: description is missing or empty")
	case utf8.RuneCountInString(desc) > maxDescriptionLen:
		out = append(out, fmt.Sprintf("SKILL.md: description is longer than %d characters", maxDescriptionLen))
	}

	return out
}

func checkReferences(s *Skill) ([]string, error) {
	var out []string
	err := filepath.WalkDir(s.Dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(s.Dir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() || !strings.EqualFold(filepath.Ext(p), ".md") {
			return nil
		}
		warns, err := scanMarkdown(s, p, rel)
		if err != nil {
			return err
		}
		out = append(out, warns...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan %s: %w", s.Dir, err)
	}
	return out, nil
}

// scanMarkdown reports the references of one Markdown file that do not resolve
// inside the skill. Fenced code blocks are skipped: they hold commands and
// samples, not links.
func scanMarkdown(s *Skill, abs, rel string) ([]string, error) {
	f, err := os.Open(abs)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", abs, err)
	}
	defer f.Close()

	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	inFence := false
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		report := func(ref string, linked bool) {
			if w := checkRef(s, rel, ref, linked); w != "" {
				out = append(out, fmt.Sprintf("%s:%d: %s", rel, n, w))
			}
		}
		for _, m := range inlineLinkRe.FindAllStringSubmatch(line, -1) {
			report(m[1], true)
		}
		if m := refDefRe.FindStringSubmatch(line); m != nil {
			report(m[1], true)
		}
		for _, m := range codeSpanRe.FindAllStringSubmatch(line, -1) {
			tok := m[1]
			if strings.ContainsAny(tok, " \t") {
				continue
			}
			// A code span is prose, so only the tokens that clearly name a
			// repository file outside the skill are checked; a bare
			// "/var/log/x" is a path inside the analyst's container.
			if strings.HasPrefix(tok, "../") || strings.Contains(tok, ".apm/") {
				report(tok, false)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", abs, err)
	}
	return out, nil
}

// checkRef returns the warning text for one reference, or "" when it resolves
// inside the skill. linked is true for Markdown links, whose targets must also
// exist.
func checkRef(s *Skill, fromRel, ref string, linked bool) string {
	target := ref
	if i := strings.IndexAny(target, "#?"); i >= 0 {
		target = target[:i]
	}
	if target == "" || schemeRe.MatchString(target) || strings.HasPrefix(target, "//") {
		return ""
	}
	if u, err := url.PathUnescape(target); err == nil {
		target = u
	}
	outside := fmt.Sprintf("%q points outside the skill folder; only .apm/skills/%s/ is installed", ref, s.Name())
	if strings.HasPrefix(target, "/") {
		return outside
	}
	resolved := path.Join(path.Dir(fromRel), target)
	if resolved == ".." || strings.HasPrefix(resolved, "../") ||
		strings.HasPrefix(resolved, ".apm/") || strings.Contains(resolved, "/.apm/") {
		return outside
	}
	if !linked {
		return ""
	}
	if _, err := os.Stat(filepath.Join(s.Dir, filepath.FromSlash(resolved))); err != nil {
		return fmt.Sprintf("%q does not exist in the skill folder", ref)
	}
	return ""
}
