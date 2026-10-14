package skills

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	maxNameLen          = 64
	maxDescriptionLen   = 1024
	maxCompatibilityLen = 500
	maxBodyLines        = 500
)

// knownFields are the frontmatter keys of the Agent Skills specification.
var knownFields = map[string]bool{
	"name":          true,
	"description":   true,
	"license":       true,
	"compatibility": true,
	"metadata":      true,
	"allowed-tools": true,
}

// nameRule returns the problem a skill name has, as the text that follows
// `name "<name>"`, or "" when the name satisfies the rule. dir is the name of
// the skill directory.
type nameRule func(name, dir string) string

var nameRules = []nameRule{
	func(name, _ string) string {
		if utf8.RuneCountInString(name) > maxNameLen {
			return fmt.Sprintf("is longer than %d characters", maxNameLen)
		}
		return ""
	},
	func(name, _ string) string {
		if strings.IndexFunc(name, notNameRune) >= 0 {
			return "may contain only lowercase letters, digits and hyphens"
		}
		return ""
	},
	func(name, _ string) string {
		if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
			return "must not start or end with a hyphen"
		}
		return ""
	},
	func(name, _ string) string {
		if strings.Contains(name, "--") {
			return "must not contain consecutive hyphens"
		}
		return ""
	},
	func(name, dir string) string {
		if name != dir {
			return fmt.Sprintf("must equal the directory name %q", dir)
		}
		return ""
	},
}

func notNameRune(r rune) bool {
	return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-')
}

func checkFrontmatter(s *Skill) []string {
	out := checkName(s.Meta.Name, s.Name())
	out = append(out, checkDescription(s.Meta.Description)...)
	return append(out, checkOptionalFields(s.Fields)...)
}

func checkName(name, dir string) []string {
	if name == "" {
		return []string{"SKILL.md: name is missing"}
	}
	var out []string
	for _, rule := range nameRules {
		if problem := rule(name, dir); problem != "" {
			out = append(out, fmt.Sprintf("SKILL.md: name %q %s", name, problem))
		}
	}
	return out
}

func checkDescription(desc string) []string {
	desc = strings.TrimSpace(desc)
	switch {
	case desc == "":
		return []string{"SKILL.md: description is missing or empty"}
	case utf8.RuneCountInString(desc) > maxDescriptionLen:
		return []string{fmt.Sprintf("SKILL.md: description is longer than %d characters", maxDescriptionLen)}
	}
	return nil
}

// checkOptionalFields checks the types and limits of the optional fields in
// the raw frontmatter and reports the keys the specification does not define.
func checkOptionalFields(fields map[string]any) []string {
	var problems []string
	for _, p := range []string{
		checkStringField(fields, "license", 0),
		checkStringField(fields, "compatibility", maxCompatibilityLen),
		checkStringField(fields, "allowed-tools", 0),
	} {
		if p != "" {
			problems = append(problems, p)
		}
	}
	problems = append(problems, checkMetadata(fields["metadata"])...)
	problems = append(problems, checkUnknownFields(fields)...)

	out := make([]string, len(problems))
	for i, p := range problems {
		out[i] = "SKILL.md: " + p
	}
	return out
}

// checkStringField returns the problem with the string field key, or "" when
// it is absent or valid. A maxLen of 0 means no length limit.
func checkStringField(fields map[string]any, key string, maxLen int) string {
	v := fields[key]
	if v == nil {
		return ""
	}
	str, ok := v.(string)
	switch {
	case !ok:
		return fmt.Sprintf("%s must be a string", key)
	case maxLen > 0 && utf8.RuneCountInString(str) > maxLen:
		return fmt.Sprintf("%s is longer than %d characters", key, maxLen)
	}
	return ""
}

// checkMetadata reports the entries of the metadata field that are not
// string-to-string.
func checkMetadata(v any) []string {
	if v == nil {
		return nil
	}
	m, ok := v.(map[any]any) // gopkg.in/yaml.v2 decodes nested maps this way
	if !ok {
		return []string{"metadata must be a map of strings to strings"}
	}
	var out []string
	for k, val := range m {
		key, ok := k.(string)
		if !ok {
			out = append(out, fmt.Sprintf("metadata key %v must be a string", k))
			continue
		}
		if _, ok := val.(string); !ok {
			out = append(out, fmt.Sprintf("metadata value of %q must be a string", key))
		}
	}
	slices.Sort(out)
	return out
}

func checkUnknownFields(fields map[string]any) []string {
	var out []string
	for key := range fields {
		if !knownFields[key] {
			out = append(out, fmt.Sprintf("unknown frontmatter field %q", key))
		}
	}
	slices.Sort(out)
	return out
}

// checkBodySize reports a SKILL.md body longer than maxBodyLines, so that the
// whole skill is not loaded into the agent's context at once.
func checkBodySize(s *Skill) []string {
	if s.BodyLines > maxBodyLines {
		return []string{fmt.Sprintf("SKILL.md: body has %d lines, more than %d", s.BodyLines, maxBodyLines)}
	}
	return nil
}
