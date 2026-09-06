package il

import (
	"strings"
	"testing"
)

const refHead = "INTENT x@1\nKIND program\nFIDELITY structure\nTARGET py\n"

func TestScanRefsBasic(t *testing.T) {
	src := refHead +
		"CONTRACT\n  R1: 错误处理见 <ref: common#R3@1.2>\n  R2: 本地规则\n" +
		"ACCEPT\n  A1: 见 <ref: common#A1@1.2>\n"
	doc, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	refs := doc.ScanRefs()
	if len(refs) != 2 {
		t.Fatalf("ScanRefs = %d, want 2", len(refs))
	}
	if refs[0].Ref.Archive != "common" || refs[0].Ref.Entry != "R3" || refs[0].Ref.Version != "1.2" {
		t.Errorf("ref0 parse wrong: %+v", refs[0].Ref)
	}
	if refs[0].Entry != "R1" {
		t.Errorf("ref0 owner = %q, want R1", refs[0].Entry)
	}
	if refs[0].Section != "CONTRACT" {
		t.Errorf("ref0 section = %q", refs[0].Section)
	}
	if refs[1].Section != "ACCEPT" || refs[1].Entry != "A1" {
		t.Errorf("ref1 location wrong: %+v", refs[1])
	}
}

func TestScanRefsIgnoresForbiddenSections(t *testing.T) {
	src := refHead +
		"CONTRACT\n  R1: 本地\n" +
		"DECISIONS\n  D1: 参考 <ref: common#R3@1.2>\n" +
		"OPEN\n  ?1: 见 <ref: common#R3@1.2>     default: x\n"
	doc, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	// ScanRefs (allowed-only) must not return the forbidden ones
	if refs := doc.ScanRefs(); len(refs) != 0 {
		t.Errorf("ScanRefs should ignore DECISIONS/OPEN refs, got %d", len(refs))
	}
	// ScanRefsAll sees them
	if refs := doc.ScanRefsAll(); len(refs) != 2 {
		t.Errorf("ScanRefsAll should see forbidden refs, got %d", len(refs))
	}
}

func TestLintRefInForbiddenSection(t *testing.T) {
	src := refHead +
		"CONTRACT\n  R1: 本地规则\n  R2: 更多规则\nACCEPT\n  A1: ok\n" +
		"DECISIONS\n  D1: 决定A 参考 <ref: common#R3@1.2>     reject: x     due: y\n"
	doc, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range doc.Lint() {
		if d.Section == "DECISIONS" && d.Severity == SevError && strings.Contains(d.Msg, "<ref:") {
			found = true
		}
	}
	if !found {
		t.Error("expected forbidden-section ref error in DECISIONS")
	}
}

func TestScanRefsInAnchor(t *testing.T) {
	src := refHead +
		"CONTRACT\n  R1: a\n  R2: b\nACCEPT\n  A1: ok\n" +
		"ANCHORS\n  in: 示例见 <ref: style#A1@0.9>\n"
	doc, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if refs := doc.ScanRefs(); len(refs) != 1 {
		t.Errorf("anchors ref should count, got %d", len(refs))
	}
}
