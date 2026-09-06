package similarity

import (
	"strings"
	"testing"
)

func TestCompareIdentical(t *testing.T) {
	src := "line1\nline2\nline3\n"
	r := Compare(src, src)
	if r.Score() != 1 {
		t.Errorf("identical score = %.3f, want 1", r.Score())
	}
	if r.Added != 0 || r.Removed != 0 {
		t.Errorf("identical should have no hunks, got +%d/-%d", r.Added, r.Removed)
	}
}

func TestCompareDisjoint(t *testing.T) {
	r := Compare("a\nb\n", "c\nd\n")
	if r.Score() != 0 {
		t.Errorf("disjoint score = %.3f, want 0", r.Score())
	}
}

func TestComparePartial(t *testing.T) {
	ref := "def add(a, b):\n    return a + b\n\ndef main():\n    pass\n"
	cand := "def add(a, b):\n    return a + b\n\ndef subtract(a, b):\n    return a - b\n"
	r := Compare(ref, cand)
	if r.Score() <= 0 || r.Score() >= 1 {
		t.Errorf("partial score out of range: %.3f", r.Score())
	}
	if r.LCS < 2 {
		t.Errorf("expected at least 2 shared lines, got %d", r.LCS)
	}
}

func TestCompareEmptyBoth(t *testing.T) {
	r := Compare("", "")
	if r.Score() != 1 {
		t.Errorf("empty both score = %.3f, want 1", r.Score())
	}
}

func TestReportString(t *testing.T) {
	r := Compare("a\n", "b\n")
	s := r.String()
	if !strings.Contains(s, "相似度") {
		t.Errorf("report missing header: %s", s)
	}
	if !strings.Contains(s, "  +") && !strings.Contains(s, "  -") {
		t.Errorf("report missing diff lines: %s", s)
	}
}

func TestPass(t *testing.T) {
	if !(Compare("x\n", "x\n").Pass(1.0)) {
		t.Error("identical should pass 1.0")
	}
	if Compare("x\n", "y\n").Pass(0.5) {
		t.Error("disjoint should not pass 0.5")
	}
}
