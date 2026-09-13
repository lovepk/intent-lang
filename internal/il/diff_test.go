package il

import (
	"sort"
	"strings"
	"testing"
)

func cmpList(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aa := append([]string(nil), a...)
	bb := append([]string(nil), b...)
	sort.Strings(aa)
	sort.Strings(bb)
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}

func TestCompareEntriesAddRemoveModify(t *testing.T) {
	before := "INTENT x@1\nKIND p\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: a加b\n  R2: b\nACCEPT\n  A1: ok\n"
	after := "INTENT x@1\nKIND p\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: a加c\n  R2: b\n  R3: 新增\nACCEPT\n  A1: ok\n"
	d := CompareEntries(before, after)
	if !cmpList(d.Added, []string{"R3"}) {
		t.Errorf("added = %v, want [R3]", d.Added)
	}
	if !cmpList(d.Modified, []string{"R1"}) {
		t.Errorf("modified = %v, want [R1]", d.Modified)
	}
	if len(d.Removed) != 0 {
		t.Errorf("removed = %v, want none", d.Removed)
	}
}

func TestCompareEntriesRemove(t *testing.T) {
	before := "INTENT x@1\nKIND p\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: a\n  R2: b\n"
	after := "INTENT x@1\nKIND p\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: a\n"
	d := CompareEntries(before, after)
	if !cmpList(d.Removed, []string{"R2"}) {
		t.Errorf("removed = %v, want [R2]", d.Removed)
	}
}

func TestCompareEntriesWhitespaceChurnNotModified(t *testing.T) {
	before := "INTENT x@1\nKIND p\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: a  b\n"
	after := "INTENT x@1\nKIND p\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: a b\n"
	d := CompareEntries(before, after)
	if len(d.Modified) != 0 {
		t.Errorf("whitespace churn should not count as modified: %v", d.Modified)
	}
}

func TestCompareEntriesIgnoresMeta(t *testing.T) {
	before := "INTENT x@1\nKIND p\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: a\nMETA\n  spec: v2.0\n  commits: 1\n"
	after := "INTENT x@1\nKIND p\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: a\nMETA\n  spec: v2.0\n  commits: 5\n  deprecated: [R2]\n"
	d := CompareEntries(before, after)
	if len(d.Changed()) != 0 {
		t.Errorf("meta-only change should not count: %v", d.Changed())
	}
}

func TestCompareEntriesCoversHeaderAnchorsSnippet(t *testing.T) {
	before := "INTENT x@1\nKIND p\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: a\nANCHORS\n  in: 1\n  out: 2\nSNIPPET core\n  print(1)\n"
	after := "INTENT x@1\nKIND p\nFIDELITY artifact\nTARGET py\nCONTRACT\n  R1: a\nANCHORS\n  in: 1\n  out: 3\nSNIPPET core\n  print(2)\n"
	d := CompareEntries(before, after)
	all := d.Changed()
	want := map[string]bool{"FIDELITY": true, "ANCHORS": true, "SNIPPET:core": true}
	if len(all) != 3 {
		t.Fatalf("changed = %v, want 3 units", all)
	}
	for _, id := range all {
		if !want[id] {
			t.Errorf("unexpected changed unit %q", id)
		}
	}
}

func TestCompareEntriesSnippetAddRemove(t *testing.T) {
	before := "INTENT x@1\nKIND p\nFIDELITY artifact\nTARGET py\nCONTRACT\n  R1: a\nSNIPPET core\n  print(1)\n"
	after := "INTENT x@1\nKIND p\nFIDELITY artifact\nTARGET py\nCONTRACT\n  R1: a\nSNIPPET core\n  print(1)\nSNIPPET extra\n  print(2)\n"
	d := CompareEntries(before, after)
	if !cmpList(d.Added, []string{"SNIPPET:extra"}) {
		t.Errorf("added = %v, want [SNIPPET:extra]", d.Added)
	}
}

func TestUndeclaredChangesScratch(t *testing.T) {
	// helper lives in cmd package; here we just sanity-check Diff.Changed ordering
	before := "INTENT x@1\nKIND p\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: a\n  R2: b\n"
	after := "INTENT x@1\nKIND p\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: a\n  R2: b2\n"
	d := CompareEntries(before, after)
	changed := d.Changed()
	if len(changed) != 1 || changed[0] != "R2" {
		t.Errorf("changed = %v, want [R2]", changed)
	}
	if !strings.Contains(changed[0], "R") {
		t.Errorf("id format wrong: %s", changed[0])
	}
}
