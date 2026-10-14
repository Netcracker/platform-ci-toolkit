package skills

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// writeFiles creates files (relative path → content) under dir.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func skillMD(name, desc string) string {
	return "---\nname: " + name + "\ndescription: " + desc + "\n---\n\n# Body\n"
}

func TestLoad(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"ok/SKILL.md":      skillMD("ok", "does things"),
		"bad/SKILL.md":     "---\nname: [unclosed\n---\n",
		"nofile/README.md": "x",
		"nofm/SKILL.md":    "# no frontmatter\n",
		"extra/SKILL.md":   "---\nname: extra\ndescription: d\nunknown: 1\nmetadata:\n  k: v\n---\n",
	})

	t.Run("valid", func(t *testing.T) {
		s, err := Load(filepath.Join(root, "ok"))
		if err != nil {
			t.Fatal(err)
		}
		if s.Meta.Name != "ok" || s.Meta.Description != "does things" || s.Name() != "ok" {
			t.Errorf("unexpected skill %+v", s)
		}
	})
	t.Run("unknown keys ignored", func(t *testing.T) {
		s, err := Load(filepath.Join(root, "extra"))
		if err != nil {
			t.Fatal(err)
		}
		if s.Meta.Metadata["k"] != "v" {
			t.Errorf("metadata not parsed: %+v", s.Meta)
		}
	})
	t.Run("missing SKILL.md", func(t *testing.T) {
		_, err := Load(filepath.Join(root, "nofile"))
		if !errors.Is(err, ErrNoSkillFile) {
			t.Errorf("got %v, want ErrNoSkillFile", err)
		}
	})
	t.Run("wrong case", func(t *testing.T) {
		dir := filepath.Join(root, "cased")
		writeFiles(t, root, map[string]string{"cased/skill.md": skillMD("cased", "d")})
		_, err := Load(dir)
		if !errors.Is(err, ErrSkillFileCase) || errors.Is(err, ErrNoSkillFile) {
			t.Errorf("got %v, want ErrSkillFileCase", err)
		}
	})
	t.Run("bad yaml", func(t *testing.T) {
		_, err := Load(filepath.Join(root, "bad"))
		if err == nil || errors.Is(err, ErrNoSkillFile) {
			t.Errorf("got %v, want parse error", err)
		}
	})
	t.Run("no frontmatter loads empty meta", func(t *testing.T) {
		s, err := Load(filepath.Join(root, "nofm"))
		if err != nil {
			t.Fatal(err)
		}
		if s.Meta.Name != "" {
			t.Errorf("unexpected meta %+v", s.Meta)
		}
	})
}

func TestLoadAll(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"a/SKILL.md":        skillMD("a", "d"),
		"b/SKILL.md":        skillMD("b", "d"),
		"broken/SKILL.md":   "---\nname: [unclosed\n---\n",
		"empty/notes.txt":   "x",
		".hidden/notes.txt": "x",
		"cased/skill.md":    skillMD("cased", "d"),
		"file.txt":          "not a dir",
	})

	loaded, skipped, err := LoadAll(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range loaded {
		names = append(names, s.Name())
	}
	if got := strings.Join(names, ","); got != "a,b" {
		t.Errorf("loaded %q, want a,b", got)
	}
	if _, ok := skipped["empty"]; ok {
		t.Error("directory without SKILL.md is not a skill and must not be reported")
	}
	if r := skipped["cased"]; !strings.Contains(r, `found "skill.md"`) {
		t.Errorf("cased reason = %q", r)
	}
	if r := skipped["broken"]; !strings.Contains(r, "parse frontmatter") || strings.Contains(r, root) {
		t.Errorf("broken reason = %q", r)
	}
	if _, ok := skipped[".hidden"]; ok {
		t.Error("hidden dir without SKILL.md must not be reported")
	}
	if _, ok := skipped["file.txt"]; ok {
		t.Error("plain file must be ignored")
	}
	if len(skipped) != 2 { // broken and cased
		t.Errorf("skipped = %v", skipped)
	}
}

func TestLoadAllMissingRoot(t *testing.T) {
	loaded, skipped, err := LoadAll(filepath.Join(t.TempDir(), "nope"))
	if loaded != nil || skipped != nil || err != nil {
		t.Errorf("got %v %v %v, want all nil", loaded, skipped, err)
	}
}

func TestLoadAllSymlinkedSkill(t *testing.T) {
	base := t.TempDir()
	writeFiles(t, base, map[string]string{"real/linked/SKILL.md": skillMD("linked", "d")})
	root := filepath.Join(base, "root")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(base, "real", "linked"), filepath.Join(root, "linked")); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlinks unavailable: %v", err)
		}
		t.Fatal(err)
	}
	loaded, _, err := LoadAll(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].Name() != "linked" {
		t.Errorf("loaded = %+v", loaded)
	}
}
