package skills

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// validate loads the skill in dir/<name> and returns its findings.
func validate(t *testing.T, root, name string) []string {
	t.Helper()
	s, err := Load(root + "/" + name)
	if err != nil {
		t.Fatal(err)
	}
	warns, err := Validate(s)
	if err != nil {
		t.Fatal(err)
	}
	return warns
}

func TestFrontmatterChecks(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		md   string
		want []string // substrings, one per expected finding, in order
	}{
		{"valid", "code-review", skillMD("code-review", "reviews code"), nil},
		{"name missing", "x", "---\ndescription: d\n---\n", []string{"name is missing"}},
		{"name 64 chars", strings.Repeat("a", 64), skillMD(strings.Repeat("a", 64), "d"), nil},
		{"name 65 chars", strings.Repeat("a", 65), skillMD(strings.Repeat("a", 65), "d"), []string{"longer than 64 characters"}},
		{"uppercase", "Abc", skillMD("Abc", "d"), []string{"only lowercase letters, digits and hyphens"}},
		{"underscore", "a_b", skillMD("a_b", "d"), []string{"only lowercase letters, digits and hyphens"}},
		{"leading hyphen", "-ab", skillMD("-ab", "d"), []string{"must not start or end with a hyphen"}},
		{"trailing hyphen", "ab-", skillMD("ab-", "d"), []string{"must not start or end with a hyphen"}},
		{"consecutive hyphens", "a--b", skillMD("a--b", "d"), []string{"consecutive hyphens"}},
		{"dir mismatch", "code-review", skillMD("review", "d"), []string{`must equal the directory name "code-review"`}},
		{"all name rules", "ok", skillMD("Code--Review-", "d"), []string{
			"only lowercase", "must not start or end", "consecutive hyphens", "must equal the directory name",
		}},
		{"description missing", "x", "---\nname: x\n---\n", []string{"description is missing or empty"}},
		{"description blank", "x", "---\nname: x\ndescription: \"   \"\n---\n", []string{"description is missing or empty"}},
		{"description 1024", "x", skillMD("x", strings.Repeat("d", 1024)), nil},
		{"description 1025", "x", skillMD("x", strings.Repeat("d", 1025)), []string{"description is longer than 1024 characters"}},
		{"multibyte name counted in runes", "x", skillMD(strings.Repeat("é", 10), "d"), []string{"only lowercase", "must equal the directory name"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, map[string]string{tc.dir + "/SKILL.md": tc.md})
			got := validate(t, root, tc.dir)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d findings %q, want %d", len(got), got, len(tc.want))
			}
			for i, w := range tc.want {
				if !strings.Contains(got[i], w) {
					t.Errorf("finding %d = %q, want it to contain %q", i, got[i], w)
				}
			}
		})
	}
}

func TestReferenceChecks(t *testing.T) {
	const front = "---\nname: s\ndescription: d\n---\n"
	tests := []struct {
		name  string
		files map[string]string
		want  []string // exact findings
	}{
		{
			name:  "valid relative link",
			files: map[string]string{"s/SKILL.md": front + "[g](references/guide.md)\n", "s/references/guide.md": "x"},
		},
		{
			name:  "image and reference definition",
			files: map[string]string{"s/SKILL.md": front + "![i](img/a.png)\n\n[ref]: docs/b.md\n", "s/img/a.png": "x", "s/docs/b.md": "x"},
		},
		{
			name:  "escaping link",
			files: map[string]string{"s/SKILL.md": front + "[g](../shared/guide.md)\n"},
			want:  []string{`SKILL.md:5: "../shared/guide.md" points outside the skill folder; only .apm/skills/s/ is installed`},
		},
		{
			name:  "absolute link",
			files: map[string]string{"s/SKILL.md": front + "[g](/etc/guide.md)\n"},
			want:  []string{`SKILL.md:5: "/etc/guide.md" points outside the skill folder; only .apm/skills/s/ is installed`},
		},
		{
			name:  "missing target",
			files: map[string]string{"s/SKILL.md": front + "[g](references/missing.md)\n"},
			want:  []string{`SKILL.md:5: "references/missing.md" does not exist in the skill folder`},
		},
		{
			name:  "anchor and query stripped",
			files: map[string]string{"s/SKILL.md": front + "[g](a.md#sec) [h](a.md?x=1)\n", "s/a.md": "x"},
		},
		{
			name:  "anchor only",
			files: map[string]string{"s/SKILL.md": front + "[g](#sec)\n"},
		},
		{
			name:  "external urls ignored",
			files: map[string]string{"s/SKILL.md": front + "[g](https://example.com/x) [m](mailto:a@b.c) [p](//cdn/x)\n"},
		},
		{
			name:  "backtick fence ignored",
			files: map[string]string{"s/SKILL.md": front + "```\n[g](../x.md)\n`../y`\n```\n"},
		},
		{
			name:  "tilde fence ignored",
			files: map[string]string{"s/SKILL.md": front + "~~~\n[g](../x.md)\n~~~\n"},
		},
		{
			name:  "fence closes",
			files: map[string]string{"s/SKILL.md": front + "```\n```\n[g](../x.md)\n"},
			want:  []string{`SKILL.md:7: "../x.md" points outside the skill folder; only .apm/skills/s/ is installed`},
		},
		{
			name:  "code span with ../",
			files: map[string]string{"s/SKILL.md": front + "See `../shared/x.md` here\n"},
			want:  []string{`SKILL.md:5: "../shared/x.md" points outside the skill folder; only .apm/skills/s/ is installed`},
		},
		{
			name:  "code span with .apm/",
			files: map[string]string{"s/SKILL.md": front + "See `.apm/skills/other/SKILL.md`\n"},
			want:  []string{`SKILL.md:5: ".apm/skills/other/SKILL.md" points outside the skill folder; only .apm/skills/s/ is installed`},
		},
		{
			name:  "code span prose ignored",
			files: map[string]string{"s/SKILL.md": front + "Run `cat /var/log/x` or `/var/log/x` or `references/none.md`\n"},
		},
		{
			name: "nested markdown relative to its own dir",
			files: map[string]string{
				"s/SKILL.md":        front,
				"s/references/a.md": "[up](../SKILL.md) [bad](../../x.md) [sib](b.md)\n",
				"s/references/b.md": "x",
			},
			want: []string{`references/a.md:1: "../../x.md" points outside the skill folder; only .apm/skills/s/ is installed`},
		},
		{
			name:  "non-markdown ignored",
			files: map[string]string{"s/SKILL.md": front, "s/notes.txt": "[g](../x.md)"},
		},
		{
			name:  "percent-encoded escape",
			files: map[string]string{"s/SKILL.md": front + "[g](%2e%2e/x.md)\n"},
			want:  []string{`SKILL.md:5: "%2e%2e/x.md" points outside the skill folder; only .apm/skills/s/ is installed`},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, tc.files)
			got := validate(t, root, "s")
			if !reflect.DeepEqual(got, tc.want) && (len(got) != 0 || len(tc.want) != 0) {
				t.Errorf("got  %q\nwant %q", got, tc.want)
			}
		})
	}
}

func TestLimit(t *testing.T) {
	const front = "---\nname: s\ndescription: d\n---\n"
	var b strings.Builder
	b.WriteString(front)
	for i := 0; i < 25; i++ {
		fmt.Fprintf(&b, "[l](../x%d.md)\n", i)
	}
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"s/SKILL.md": b.String()})
	got := validate(t, root, "s")
	if len(got) != 20 {
		t.Fatalf("got %d findings, want 20", len(got))
	}
	if !strings.Contains(got[18], "x18.md") {
		t.Errorf("finding 19 = %q", got[18])
	}
	if got[19] != "... and 6 more problems" {
		t.Errorf("tail = %q", got[19])
	}

	if got := limit(make([]string, 20)); len(got) != 20 || strings.Contains(got[19], "more problems") {
		t.Errorf("exactly 20 must pass through, got %d", len(got))
	}
}

func TestValidateAllAndWarningLines(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"good/SKILL.md": skillMD("good", "d"),
		"bad/SKILL.md":  skillMD("wrong", "d"),
		"torn/SKILL.md": "---\nname: [x\n---\n",
	})
	loaded, skipped, err := LoadAll(root)
	if err != nil {
		t.Fatal(err)
	}
	found, err := ValidateAll(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := found["good"]; ok {
		t.Error("skill without findings must have no entry")
	}
	all := WithSkipped(found, skipped)
	if _, ok := found["torn"]; ok {
		t.Error("WithSkipped must not modify its input")
	}
	if !all["torn"].Skipped || all["bad"].Skipped {
		t.Errorf("skipped flags wrong: %+v", all)
	}
	lines := WarningLines(all)
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "bad: SKILL.md: name") || !strings.HasPrefix(lines[1], "torn: skipped: parse frontmatter") {
		t.Errorf("lines = %q", lines)
	}
}
