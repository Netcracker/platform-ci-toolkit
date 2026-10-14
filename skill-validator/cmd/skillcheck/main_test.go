package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

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

func md(name string) string {
	return "---\nname: " + name + "\ndescription: does things\n---\n\n# Body\n"
}

// execute runs the CLI in-process with the NO_COLOR variable set to noColor
// and returns stdout, stderr and the exit code.
func executeEnv(t *testing.T, noColor string, args ...string) (string, string, int) {
	t.Helper()
	t.Setenv("NO_COLOR", noColor)
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut)
	return out.String(), errOut.String(), code
}

// execute runs the CLI in-process without colors.
func execute(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	return executeEnv(t, "1", args...)
}

// bundle builds a bundle root with one valid and one invalid skill.
func bundle(t *testing.T) string {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"good/SKILL.md": md("good"),
		"bad/SKILL.md":  md("wrong"),
	})
	return root
}

func TestSingleSkillFolder(t *testing.T) {
	root := bundle(t)
	out, _, code := execute(t, filepath.Join(root, "good"))
	if code != 0 || !strings.HasPrefix(out, "PASS  good\n") || !strings.Contains(out, "Summary: 1 skill checked, 1 valid") {
		t.Errorf("code=%d out=%s", code, out)
	}
}

func TestBundleRootListsEverySkill(t *testing.T) {
	root := bundle(t)
	out, _, code := execute(t, root)
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	for _, want := range []string{
		"FAIL  bad\n        SKILL.md: name \"wrong\" must equal the directory name \"bad\"\n",
		"PASS  good\n",
		"Summary: 2 skills checked, 1 valid, 1 failed, 0 skipped, 1 problem\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if !strings.HasSuffix(out, "1 problem\n") {
		t.Errorf("summary must be last:\n%s", out)
	}
}

func TestRepeatedArguments(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	writeFiles(t, a, map[string]string{"one/SKILL.md": md("one")})
	writeFiles(t, b, map[string]string{"two/SKILL.md": md("nope")})
	out, _, code := execute(t, a, b)
	if code != 1 || !strings.Contains(out, "2 skills checked") || !strings.Contains(out, "PASS  one") || !strings.Contains(out, "FAIL  two") {
		t.Errorf("code=%d out=%s", code, out)
	}
}

func TestSameFolderTwiceCountedOnce(t *testing.T) {
	root := bundle(t)
	out, _, _ := execute(t, root, root, filepath.Join(root, "."))
	if !strings.Contains(out, "2 skills checked") {
		t.Errorf("duplicates not removed:\n%s", out)
	}
}

func TestSameSkillNameInTwoFolders(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	writeFiles(t, a, map[string]string{"dup/SKILL.md": "---\nname: dup\n---\n"})
	writeFiles(t, b, map[string]string{"dup/SKILL.md": md("dup")})
	out, _, _ := execute(t, a, b)
	if !strings.Contains(out, "FAIL  dup ("+filepath.Join(a, "dup")+")") || !strings.Contains(out, "PASS  dup ("+filepath.Join(b, "dup")+")") {
		t.Errorf("clashing names not disambiguated:\n%s", out)
	}
}

func TestNoArgumentsValidatesCurrentDirectoryAsOneSkill(t *testing.T) {
	root := bundle(t)
	// A skill directory that also holds a nested skill: only the directory
	// itself is validated.
	writeFiles(t, root, map[string]string{"good/nested/SKILL.md": md("wrong")})
	t.Chdir(filepath.Join(root, "good"))
	out, errOut, code := execute(t)
	if code != 0 || !strings.Contains(out, "1 skill checked") || errOut != "" {
		t.Errorf("code=%d out=%s err=%s", code, out, errOut)
	}
}

func TestNoArgumentsSearchesBundle(t *testing.T) {
	// The current directory is a bundle root: its skill subdirectories are
	// validated, same as when "." is passed.
	root := bundle(t)
	t.Chdir(root)
	out, errOut, code := execute(t)
	dotOut, _, _ := execute(t, ".")
	if code != 1 || !strings.Contains(out, "2 skills checked") || errOut != "" {
		t.Errorf("code=%d out=%s err=%s", code, out, errOut)
	}
	if out != dotOut {
		t.Errorf("no arguments differs from \".\":\n%s\nvs\n%s", out, dotOut)
	}
}

func TestNoArgumentsWithoutSkills(t *testing.T) {
	t.Chdir(t.TempDir())
	out, errOut, code := execute(t)
	want := "Error: no skills found in . (expected SKILL.md or subdirectories containing SKILL.md)\n"
	if code != 2 || out != "" || errOut != want {
		t.Errorf("code=%d out=%q err=%q, want err %q", code, out, errOut, want)
	}
}

func TestFolderWithoutSkills(t *testing.T) {
	// Source-code-like folder: subdirectories exist, none is a skill.
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"cmd/main.go": "x", "internal/a.go": "x", "dist/b.exe": "x"})
	out, errOut, code := execute(t, root)
	want := "Error: no skills found in " + root + " (expected SKILL.md or subdirectories containing SKILL.md)\n"
	if code != 2 || out != "" || errOut != want {
		t.Errorf("code=%d out=%q err=%q, want err %q", code, out, errOut, want)
	}
}

func TestSubdirectoriesWithoutSkillFileAreIgnored(t *testing.T) {
	root := bundle(t)
	writeFiles(t, root, map[string]string{"cmd/main.go": "x", "docs/readme.txt": "x"})
	out, _, _ := execute(t, root)
	if !strings.Contains(out, "2 skills checked") || strings.Contains(out, "cmd") || strings.Contains(out, "docs") {
		t.Errorf("non-skill directories must be ignored:\n%s", out)
	}
}

func TestWrongCaseSkillFile(t *testing.T) {
	t.Run("skill folder", func(t *testing.T) {
		root := t.TempDir()
		writeFiles(t, root, map[string]string{"s/skill.md": md("s")})
		out, errOut, code := execute(t, filepath.Join(root, "s"))
		if code != 1 || errOut != "" {
			t.Errorf("code=%d err=%q", code, errOut)
		}
		for _, want := range []string{"WARN  s\n", `found "skill.md", but the file must be named SKILL.md`} {
			if !strings.Contains(out, want) {
				t.Errorf("output lacks %q:\n%s", want, out)
			}
		}
	})
	t.Run("inside bundle", func(t *testing.T) {
		root := bundle(t)
		writeFiles(t, root, map[string]string{"cased/Skill.MD": md("cased")})
		out, _, code := execute(t, root)
		if code != 1 || !strings.Contains(out, "3 skills checked") || !strings.Contains(out, "WARN  cased") || !strings.Contains(out, `found "Skill.MD"`) {
			t.Errorf("code=%d out=%s", code, out)
		}
	})
	t.Run("no arguments", func(t *testing.T) {
		root := t.TempDir()
		writeFiles(t, root, map[string]string{"skill.md": md("x")})
		t.Chdir(root)
		out, _, code := execute(t)
		if code != 1 || !strings.Contains(out, `found "skill.md"`) {
			t.Errorf("code=%d out=%s", code, out)
		}
	})
}

func TestDotInsideSkillUsesRealFolderName(t *testing.T) {
	root := bundle(t)
	t.Chdir(filepath.Join(root, "good"))
	out, _, code := execute(t, ".")
	if code != 0 || !strings.HasPrefix(out, "PASS  good\n") {
		t.Errorf("code=%d out=%s", code, out)
	}
}

func TestMissingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	file := filepath.Join(t.TempDir(), "f.txt")
	writeFiles(t, filepath.Dir(file), map[string]string{"f.txt": "x"})
	out, errOut, code := execute(t, missing, file)
	if code != 2 || out != "" {
		t.Errorf("code=%d out=%q", code, out)
	}
	if !strings.Contains(errOut, missing) || !strings.Contains(errOut, file+": not a directory") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestExitCodes(t *testing.T) {
	clean := t.TempDir()
	writeFiles(t, clean, map[string]string{"ok/SKILL.md": md("ok")})
	broken := t.TempDir()
	writeFiles(t, broken, map[string]string{"torn/SKILL.md": "---\nname: [x\n---\n"})

	tests := []struct {
		name string
		args []string
		code int
		out  string
	}{
		{"clean", []string{clean}, 0, "Summary: 1 skill checked, 1 valid, 0 failed, 0 skipped, 0 problems"},
		{"failed", []string{bundle(t)}, 1, "FAIL  bad"},
		{"skipped skill", []string{broken}, 1, "WARN  torn"},
		{"unparsable skill file as single skill", []string{filepath.Join(broken, "torn")}, 1, "WARN  torn"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, _, code := execute(t, tc.args...)
			if code != tc.code || !strings.Contains(out, tc.out) {
				t.Errorf("code=%d out=%s", code, out)
			}
		})
	}

	t.Run("unknown flag", func(t *testing.T) {
		out, errOut, code := execute(t, "--bogus")
		if code != 2 || out != "" || !strings.Contains(errOut, "bogus") || !strings.Contains(errOut, "USAGE") {
			t.Errorf("code=%d out=%q err=%q", code, out, errOut)
		}
	})
}

func TestColors(t *testing.T) {
	root := bundle(t)

	t.Run("on by default, also when piped", func(t *testing.T) {
		out, _, _ := executeEnv(t, "", root)
		for _, want := range []string{"\x1b[32mPASS\x1b[0m  good", "\x1b[31mFAIL\x1b[0m  bad"} {
			if !strings.Contains(out, want) {
				t.Errorf("output lacks %q:\n%q", want, out)
			}
		}
	})
	t.Run("NO_COLOR", func(t *testing.T) {
		if out, _, _ := executeEnv(t, "1", root); strings.Contains(out, "\x1b") {
			t.Errorf("escape sequence with NO_COLOR:\n%q", out)
		}
	})
	t.Run("--no-color", func(t *testing.T) {
		if out, _, _ := executeEnv(t, "", "--no-color", root); strings.Contains(out, "\x1b") {
			t.Errorf("escape sequence with --no-color:\n%q", out)
		}
	})
}

func TestHelp(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		out, errOut, code := execute(t, flag)
		if code != 0 || errOut != "" {
			t.Errorf("%s: code=%d err=%q", flag, code, errOut)
		}
		for _, want := range []string{"USAGE", "ARGUMENTS", "OUTPUT", "EXIT CODES", "CHECKS", "PASS", "FAIL", "WARN", "NO_COLOR", "--no-color", "  0 ", "  1 ", "  2 "} {
			if !strings.Contains(out, want) {
				t.Errorf("%s: help lacks %q", flag, want)
			}
		}
	}
}

func TestVersion(t *testing.T) {
	out, _, code := execute(t, "--version")
	if code != 0 || !strings.HasPrefix(out, "skillcheck ") {
		t.Errorf("code=%d out=%q", code, out)
	}
}

// TestBinary builds the real executable and runs it against fixture skills.
func TestBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	exe := filepath.Join(t.TempDir(), "skillcheck")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", exe, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"valid/SKILL.md":        md("valid"),
		"invalid/SKILL.md":      md("invalid") + "[x](../outside.md)\n",
		"torn/SKILL.md":         "---\nname: [x\n---\n",
		"valid/references/a.md": "text\n",
		"valid/extra.md":        "[ok](references/a.md)\n",
		"docs/notes.txt":        "not a skill",
	})

	cmd := exec.Command(exe, "--no-color", root)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("err = %v, want exit code 1\nstderr: %s", err, errOut.String())
	}
	for _, want := range []string{
		"FAIL  invalid\n", `"../outside.md" points outside`,
		"WARN  torn\n",
		"PASS  valid\n",
		"Summary: 3 skills checked, 1 valid, 1 failed, 1 skipped, 2 problems\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("stdout lacks %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "docs") {
		t.Errorf("non-skill directory reported:\n%s", out.String())
	}

	// A clean run exits 0.
	if out, err := exec.Command(exe, filepath.Join(root, "valid")).CombinedOutput(); err != nil {
		t.Errorf("clean run failed: %v\n%s", err, out)
	}
}
