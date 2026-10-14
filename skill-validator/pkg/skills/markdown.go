package skills

import (
	"bufio"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	inlineLinkRe = regexp.MustCompile("!?\\[[^]]*]\\(\\s*<?([^)\\s>]+)>?(?:\\s+\"[^\"]*\")?\\s*\\)")
	refDefRe     = regexp.MustCompile(`^\s{0,3}\[[^]]+]:\s*<?([^\s>]+)>?`)
	codeSpanRe   = regexp.MustCompile("`([^`\\n]+)`")
	schemeRe     = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)
)

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

// fenceTracker follows the fenced code blocks of a Markdown file, line by line.
// A block ends with a fence of the same character that is at least as long as
// the opening one.
type fenceTracker struct {
	marker   byte // '`' or '~' of the open fence
	size     int  // Length of the open fence; 0 when no fence is open
	openLine int  // Line number of the open fence
}

// next consumes line, number lineNo, and reports whether it belongs to a fenced
// block: a fence delimiter or a line inside the block.
func (f *fenceTracker) next(line string, lineNo int) bool {
	marker, size, rest := fenceOf(line)
	switch {
	case f.size > 0 && marker == f.marker && size >= f.size && rest == "":
		f.size = 0
		return true
	case f.size > 0:
		return true
	case size > 0:
		f.marker, f.size, f.openLine = marker, size, lineNo
		return true
	}
	return false
}

// unclosed returns the line of the fence that was opened and never closed.
func (f *fenceTracker) unclosed() (line int, ok bool) {
	return f.openLine, f.size > 0
}

// fenceOf returns the character and length of the fence that line starts with,
// and the text after it. The length is 0 when the line is not a fence.
func fenceOf(line string) (marker byte, size int, rest string) {
	t := strings.TrimSpace(line)
	if t == "" || (t[0] != '`' && t[0] != '~') {
		return 0, 0, ""
	}
	marker = t[0]
	for size < len(t) && t[size] == marker {
		size++
	}
	rest = strings.TrimSpace(t[size:])
	if size < 3 || (marker == '`' && strings.Contains(rest, "`")) {
		return 0, 0, ""
	}
	return marker, size, rest
}

// ref is a reference to a file found in a line of Markdown.
type ref struct {
	target string
	linked bool // A Markdown link, whose target must also exist
}

// lineRefs returns the references in one line outside a fenced block.
func lineRefs(line string) []ref {
	var out []ref
	for _, m := range inlineLinkRe.FindAllStringSubmatch(line, -1) {
		out = append(out, ref{m[1], true})
	}
	if m := refDefRe.FindStringSubmatch(line); m != nil {
		out = append(out, ref{m[1], true})
	}
	for _, m := range codeSpanRe.FindAllStringSubmatch(line, -1) {
		if isRepoFileSpan(m[1]) {
			out = append(out, ref{m[1], false})
		}
	}
	return out
}

// isRepoFileSpan reports whether a code span names a repository file outside
// the skill. A code span is prose, so only the tokens that clearly do are
// checked; a bare "/var/log/x" is a path inside the analyst's container.
func isRepoFileSpan(tok string) bool {
	return !strings.ContainsAny(tok, " \t") &&
		(strings.HasPrefix(tok, "../") || strings.Contains(tok, ".apm/"))
}

// scanMarkdown reports the references of one Markdown file that do not resolve
// inside the skill, and a fenced code block that is never closed. Fenced code
// blocks are skipped: they hold commands and samples, not links.
func scanMarkdown(s *Skill, abs, rel string) ([]string, error) {
	f, err := os.Open(abs)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", abs, err)
	}
	defer f.Close()

	var out []string
	var fence fenceTracker
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if fence.next(line, n) {
			continue
		}
		for _, r := range lineRefs(line) {
			if w := checkRef(s, rel, r.target, r.linked); w != "" {
				out = append(out, fmt.Sprintf("%s:%d: %s", rel, n, w))
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", abs, err)
	}
	if line, ok := fence.unclosed(); ok {
		out = append(out, fmt.Sprintf("%s:%d: code fence is not closed", rel, line))
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
