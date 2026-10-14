// Package skills loads and validates Agent Skills from disk.
//
// Agent Skills is an open standard for packaging procedural knowledge
// as a directory containing a SKILL.md with YAML frontmatter. The spec
// is published at https://agentskills.io/specification.
//
// Two read entry points:
//
//   - Load(dir): load one skill from its directory. Returns
//     ErrNoSkillFile if the directory does not contain a SKILL.md so
//     that callers walking heterogeneous directories can decide
//     whether that is a hard failure or a soft skip.
//   - LoadAll(root): load every skill under root by iterating over
//     immediate subdirectories that contain a SKILL.md and calling Load on
//     each, reporting the ones that cannot be loaded. Subdirectories without
//     any SKILL.md are not skills and are ignored. Missing root →
//     (nil, nil, nil).
//
// Validation is a separate step on already loaded skills: Validate for one
// skill, ValidateAll for many.
package skills

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/frontmatter"
)

// SkillFile is the canonical filename inside an Agent Skills directory.
// The Agent Skills specification mandates this exact name.
const SkillFile = "SKILL.md"

// Meta is the SKILL.md YAML frontmatter as defined by the Agent Skills
// specification. See https://agentskills.io/specification for
// authoritative field definitions and constraints. Unknown keys are
// ignored on read.
type Meta struct {
	// Name (required). Max 64 chars, lowercase letters/digits/hyphens
	// only, must not start or end with `-` or contain `--`, and must
	// equal the parent directory's name.
	Name string `yaml:"name"`

	// Description (required). Max 1024 chars, non-empty.
	Description string `yaml:"description"`

	// License (optional).
	License string `yaml:"license,omitempty"`

	// Compatibility (optional). Max 500 chars.
	Compatibility string `yaml:"compatibility,omitempty"`

	// Metadata (optional). String-to-string map.
	Metadata map[string]string `yaml:"metadata,omitempty"`

	// AllowedTools (optional, experimental). Space-separated list.
	AllowedTools string `yaml:"allowed-tools,omitempty"`
}

// Skill is one loaded agent skill.
type Skill struct {
	Dir  string // Absolute path to the skill's directory.
	Meta Meta   // Parsed SKILL.md frontmatter.
}

// Name returns the skill's directory basename.
//
// The Agent Skills spec requires this to equal Meta.Name; callers that
// want the on-disk identifier should prefer this method.
func (s *Skill) Name() string {
	return filepath.Base(s.Dir)
}

// ErrNoSkillFile is returned by Load when the directory exists but
// does not contain a SKILL.md. Use errors.Is to detect it.
var ErrNoSkillFile = errors.New("SKILL.md not found")

// ErrSkillFileCase is returned by Load when the directory has a file whose
// name matches SKILL.md only when case is ignored, such as skill.md. The
// specification requires the exact name, and case-insensitive file systems
// would otherwise hide the defect until the skill is used elsewhere.
var ErrSkillFileCase = errors.New("SKILL.md has the wrong case")

// caseError is the error behind ErrSkillFileCase; it names the file found.
type caseError struct{ found string }

func (e *caseError) Error() string {
	return fmt.Sprintf("found %q, but the file must be named %s", e.found, SkillFile)
}

func (e *caseError) Is(target error) bool { return target == ErrSkillFileCase }

// FindSkillFile looks in dir for the skill file by listing the directory, so
// that the answer does not depend on the case sensitivity of the file system.
// It returns the name of the file with the exact name SKILL.md, or else of a
// file that differs from it only in case, or else "". exact tells which one.
func FindSkillFile(dir string) (name string, exact bool, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch {
		case e.Name() == SkillFile:
			return e.Name(), true, nil
		case strings.EqualFold(e.Name(), SkillFile) && name == "":
			name = e.Name()
		}
	}
	return name, false, nil
}

// Load reads the skill rooted at dir.
//
// Parses dir/SKILL.md and returns the resulting *Skill. Returns
// ErrNoSkillFile (wrapped, detect via errors.Is) when SKILL.md is
// missing and ErrSkillFileCase when only a differently cased name such as
// skill.md exists; any other I/O error or YAML parse error is returned
// wrapped.
//
// Load does not validate spec rules beyond YAML well-formedness. Use
// Validate for that.
func Load(dir string) (*Skill, error) {
	found, exact, err := FindSkillFile(dir)
	switch {
	case err != nil:
		return nil, fmt.Errorf("read %s: %w", dir, err)
	case found == "":
		return nil, fmt.Errorf("%s: %w", dir, ErrNoSkillFile)
	case !exact:
		return nil, &caseError{found}
	}
	path := filepath.Join(dir, SkillFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var meta Meta
	if _, err := frontmatter.Parse(bytes.NewReader(data), &meta); err != nil {
		return nil, fmt.Errorf("parse frontmatter %s: %w", path, err)
	}
	return &Skill{Dir: dir, Meta: meta}, nil
}

// LoadAll walks root and returns one *Skill per immediate subdirectory
// that contains a loadable SKILL.md.
//
// A subdirectory with a SKILL.md that cannot be loaded (unparsable frontmatter,
// or a wrongly cased file name such as skill.md) is not loaded; skipped maps
// its name to the reason, so one broken skill never hides the others. A
// subdirectory without any SKILL.md is not a skill and is ignored. Only an
// I/O failure on root itself is an error.
//
// If root itself does not exist, LoadAll returns (nil, nil, nil).
//
// Entries are followed through symlinks: a symlink pointing at a skill
// directory is treated as that directory.
//
// Result order matches os.ReadDir (lexicographic).
func LoadAll(root string) (loaded []*Skill, skipped map[string]string, err error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("read skills root %s: %w", root, err)
	}
	skip := func(name, reason string) {
		if skipped == nil {
			skipped = make(map[string]string)
		}
		skipped[name] = reason
	}

	for _, e := range entries {
		path := filepath.Join(root, e.Name())
		if !e.IsDir() {
			info, err := os.Stat(path)
			if err != nil || !info.IsDir() {
				continue
			}
		}
		s, err := Load(path)
		switch {
		case errors.Is(err, ErrNoSkillFile):
			// Not a skill directory.
		case err != nil:
			skip(e.Name(), strings.ReplaceAll(err.Error(), path+string(os.PathSeparator), ""))
		default:
			loaded = append(loaded, s)
		}
	}
	return loaded, skipped, nil
}
