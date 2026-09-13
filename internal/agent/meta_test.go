package agent

import (
	"strings"
	"testing"
)

func TestBuildMetaLines(t *testing.T) {
	lines := BuildMetaLines(3, "2026-01-01T00:00:00Z", []string{"R2", "R5"})
	got := strings.Join(lines, "\n")
	for _, want := range []string{
		"spec: v2.0",
		"created: 2026-01-01T00:00:00Z",
		"commits: 3",
		"deprecated: [R2, R5]",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("BuildMetaLines missing %q:\n%s", want, got)
		}
	}
}

func TestBuildMetaLinesEmptyDeprecated(t *testing.T) {
	lines := BuildMetaLines(1, "", nil)
	got := strings.Join(lines, "\n")
	if !strings.Contains(got, "deprecated: []") {
		t.Errorf("expected empty deprecated, got:\n%s", got)
	}
	if !strings.Contains(got, "commits: 1") {
		t.Errorf("expected commits 1, got:\n%s", got)
	}
	if !strings.Contains(got, "created:") {
		t.Error("expected created to be auto-filled")
	}
}

func TestNextDeprecated(t *testing.T) {
	before := "INTENT x@1\nKIND p\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: a\n  R2: b\n  R3: c\n"
	after := "INTENT x@1\nKIND p\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: a\n  R3: c\n"
	got := NextDeprecated(nil, before, after)
	if len(got) != 1 || got[0] != "R2" {
		t.Errorf("NextDeprecated = %v, want [R2]", got)
	}

	// accumulate: R2 already deprecated, now R3 removed too
	after2 := "INTENT x@1\nKIND p\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: a\n"
	got2 := NextDeprecated([]string{"R2"}, after, after2)
	joined := strings.Join(got2, ",")
	if joined != "R2,R3" {
		t.Errorf("accumulated deprecated = %v, want [R2 R3]", got2)
	}
}

func TestNextDeprecatedNoChange(t *testing.T) {
	before := "INTENT x@1\nKIND p\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: a\n  R2: b\n"
	after := "INTENT x@1\nKIND p\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: a\n  R2: b\n  R3: c\n"
	got := NextDeprecated([]string{"R9"}, before, after)
	if len(got) != 1 || got[0] != "R9" {
		t.Errorf("adding R3 must not deprecate anything, got %v", got)
	}
}

func TestParseMetaCarry(t *testing.T) {
	text := "INTENT x@1\nKIND p\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: a\nMETA\n  spec: v2.0\n  created: 2026-02-02T00:00:00Z\n  commits: 4\n  deprecated: [R2, R7]\n"
	created, dep := ParseMetaCarry(text)
	if created != "2026-02-02T00:00:00Z" {
		t.Errorf("created = %q", created)
	}
	if strings.Join(dep, ",") != "R2,R7" {
		t.Errorf("deprecated = %v", dep)
	}
}

func TestArtifactExt(t *testing.T) {
	cases := map[string]string{
		"python@3.12 single-file": ".py",
		"go binary":               ".go",
		"node js":                 ".js",
		"markdown doc":            ".md",
		"plain text":              "",
	}
	for in, want := range cases {
		if got := ArtifactExt(in); got != want {
			t.Errorf("ArtifactExt(%q) = %q, want %q", in, got, want)
		}
	}
}
