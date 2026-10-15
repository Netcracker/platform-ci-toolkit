package skills

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// validate loads the skill in dir/<name> and returns its findings.
func validate(t *testing.T, root, name string, opts ...Option) []string {
	t.Helper()
	s, err := Load(root + "/" + name)
	if err != nil {
		t.Fatal(err)
	}
	problems, err := NewValidator(opts...).Validate(s)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, p := range problems {
		if p.Severity != SeverityError {
			t.Errorf("problem %q has severity %v, want error", p.Details, p.Severity)
		}
		out = append(out, p.Details)
	}
	return out
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
			name: "links inside code spans are samples",
			files: map[string]string{"s/SKILL.md": front +
				"- Use links: `[Feature Name](/docs/public/feature-name.md)`.\n" +
				"- Images: `![Alt Text](/docs/public/images/a.png)`\n" +
				"- Appears as: `[Full CR analysis\\|<url>]`\n"},
		},
		{
			name:  "real link next to code span still checked",
			files: map[string]string{"s/SKILL.md": front + "`[a](/x.md)` and [g](references/missing.md)\n"},
			want:  []string{`SKILL.md:5: "references/missing.md" does not exist in the skill folder`},
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
	if got := validate(t, root, "s"); len(got) != 25 {
		t.Errorf("default must be unlimited, got %d findings, want 25", len(got))
	}
	if got := validate(t, root, "s", WithMaxProblems(-3)); len(got) != 25 {
		t.Errorf("negative must be unlimited, got %d findings, want 25", len(got))
	}
	if got := validate(t, root, "s", WithMaxProblems(25)); len(got) != 25 || strings.Contains(got[24], "more problems") {
		t.Errorf("exactly the limit must pass through, got %d", len(got))
	}

	got := validate(t, root, "s", WithMaxProblems(20))
	if len(got) != 20 {
		t.Fatalf("got %d findings, want 20", len(got))
	}
	if !strings.Contains(got[18], "x18.md") {
		t.Errorf("finding 19 = %q", got[18])
	}
	if got[19] != "... and 6 more problems" {
		t.Errorf("tail = %q", got[19])
	}
	if got := validate(t, root, "s", WithMaxProblems(1)); !reflect.DeepEqual(got, []string{"... and 25 more problems"}) {
		t.Errorf("limit 1 = %q", got)
	}
}

func TestLimitFunc(t *testing.T) {
	if got := limit(make([]Problem, 20), 20); len(got) != 20 || strings.Contains(got[19].Details, "more problems") {
		t.Errorf("exactly n must pass through, got %d", len(got))
	}
	if got := limit(make([]Problem, 21), 20); got[19].Severity != SeverityError {
		t.Errorf("tail severity = %v, want error", got[19].Severity)
	}
	if got := limit(make([]Problem, 50), 0); len(got) != 50 {
		t.Errorf("zero must not limit, got %d", len(got))
	}
}

func TestSeverity(t *testing.T) {
	if (Problem{}).Severity != SeverityError {
		t.Error("zero severity must be error")
	}
	if SeverityError.String() != "error" || SeverityWarning.String() != "warning" || Severity(9).String() != "severity(9)" {
		t.Error("unexpected Severity.String")
	}
	if got := (Problem{Details: "x"}).String(); got != "x" {
		t.Errorf("Problem.String = %q", got)
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
	found, err := NewValidator().ValidateAll(loaded)
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

func TestOptionalFieldChecks(t *testing.T) {
	const head = "---\nname: s\ndescription: d\n"
	tests := []struct {
		name string
		fm   string // extra frontmatter lines
		want []string
	}{
		{"valid optional fields", "license: MIT\ncompatibility: needs git\nallowed-tools: Bash Read\nmetadata:\n  k: v\n", nil},
		{"license not a string", "license: 123\n", []string{"SKILL.md: license must be a string"}},
		{"allowed-tools not a string", "allowed-tools: 5\n", []string{"SKILL.md: allowed-tools must be a string"}},
		{"compatibility 500", "compatibility: " + strings.Repeat("c", 500) + "\n", nil},
		{"compatibility 501", "compatibility: " + strings.Repeat("c", 501) + "\n", []string{"SKILL.md: compatibility is longer than 500 characters"}},
		{"metadata non-string value", "metadata:\n  b: 1\n  a: true\n  c: ok\n", []string{
			`SKILL.md: metadata value of "a" must be a string`,
			`SKILL.md: metadata value of "b" must be a string`,
		}},
		{"metadata non-string key", "metadata:\n  1: x\n", []string{"SKILL.md: metadata key 1 must be a string"}},
		{"unknown fields sorted", "zeta: 1\nalpha: 2\n", []string{
			`SKILL.md: unknown frontmatter field "alpha"`,
			`SKILL.md: unknown frontmatter field "zeta"`,
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, map[string]string{"s/SKILL.md": head + tc.fm + "---\n"})
			got := validate(t, root, "s")
			if !reflect.DeepEqual(got, tc.want) && (len(got) != 0 || len(tc.want) != 0) {
				t.Errorf("got  %q\nwant %q", got, tc.want)
			}
		})
	}
}

func TestBodySize(t *testing.T) {
	const front = "---\nname: s\ndescription: d\n---\n"
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"empty body", "", nil},
		{"500 lines", strings.Repeat("x\n", 500), nil},
		{"500 lines without final newline", strings.Repeat("x\n", 499) + "x", nil},
		{"501 lines", strings.Repeat("x\n", 501), []string{"SKILL.md: body has 501 lines, more than 500"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, map[string]string{"s/SKILL.md": front + tc.body})
			got := validate(t, root, "s")
			if !reflect.DeepEqual(got, tc.want) && (len(got) != 0 || len(tc.want) != 0) {
				t.Errorf("got  %q\nwant %q", got, tc.want)
			}
		})
	}
}

func TestUnclosedFence(t *testing.T) {
	const front = "---\nname: s\ndescription: d\n---\n"
	tests := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{
			name:  "unclosed backticks",
			files: map[string]string{"s/SKILL.md": front + "text\n```sh\necho\n"},
			want:  []string{"SKILL.md:6: code fence is not closed"},
		},
		{
			name:  "unclosed tildes",
			files: map[string]string{"s/SKILL.md": front + "~~~\necho\n"},
			want:  []string{"SKILL.md:5: code fence is not closed"},
		},
		{
			name:  "tildes do not close backticks",
			files: map[string]string{"s/SKILL.md": front + "```\n~~~\n"},
			want:  []string{"SKILL.md:5: code fence is not closed"},
		},
		{
			name:  "shorter fence does not close",
			files: map[string]string{"s/SKILL.md": front + "````\n```\n"},
			want:  []string{"SKILL.md:5: code fence is not closed"},
		},
		{
			name:  "longer fence closes",
			files: map[string]string{"s/SKILL.md": front + "```\nx\n````\n"},
		},
		{
			name:  "closing fence with text does not close",
			files: map[string]string{"s/SKILL.md": front + "```\n``` x\n"},
			want:  []string{"SKILL.md:5: code fence is not closed"},
		},
		{
			name:  "inline triple backticks are not a fence",
			files: map[string]string{"s/SKILL.md": front + "```code``` in prose\n"},
		},
		{
			name: "nested reference file",
			files: map[string]string{
				"s/SKILL.md":        front,
				"s/references/a.md": "intro\n```\nopen\n",
			},
			want: []string{"references/a.md:2: code fence is not closed"},
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

func TestFenceTracker(t *testing.T) {
	var f fenceTracker
	steps := []struct {
		line string
		skip bool
	}{
		{"prose", false},
		{"  ```go", true},
		{"[g](../x.md)", true},
		{"```", true},
		{"[g](../x.md)", false},
	}
	for i, s := range steps {
		if got := f.next(s.line, i+1); got != s.skip {
			t.Errorf("line %d %q: skip = %v, want %v", i+1, s.line, got, s.skip)
		}
	}
	if _, ok := f.unclosed(); ok {
		t.Error("fence must be closed")
	}
}

func TestLineRefs(t *testing.T) {
	got := lineRefs("[a](x.md) ![i](y.png) `../z.md` `cat ../w` `/var/log` `.apm/s/SKILL.md`")
	want := []ref{{"x.md", true}, {"y.png", true}, {"../z.md", false}, {".apm/s/SKILL.md", false}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
	if got := lineRefs("`[a](/x.md)` `![i](/y.png)` [`c`](z.md)"); !reflect.DeepEqual(got, []ref{{"z.md", true}}) {
		t.Errorf("links in code spans: got %v", got)
	}
	if got := lineRefs("[ref]: docs/b.md"); !reflect.DeepEqual(got, []ref{{"docs/b.md", true}}) {
		t.Errorf("ref definition: got %v", got)
	}
}

// A field whose YAML shape cannot be decoded into Meta makes the skill
// unloadable, so it is reported as skipped rather than as a finding.
func TestStructuralFieldMismatchSkipsSkill(t *testing.T) {
	for _, fm := range []string{"license: [a, b]\n", "allowed-tools: [Bash]\n", "metadata: text\n"} {
		root := t.TempDir()
		writeFiles(t, root, map[string]string{"s/SKILL.md": "---\nname: s\ndescription: d\n" + fm + "---\n"})
		if _, err := Load(root + "/s"); err == nil {
			t.Errorf("Load accepted %q", fm)
		}
	}
}
