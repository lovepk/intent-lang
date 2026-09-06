package il

import (
	"strings"
	"testing"
)

func parseOK(t *testing.T, src string) *Doc {
	t.Helper()
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return doc
}

const baseHead = "INTENT x@1\nKIND program\nFIDELITY behavior\nTARGET py\n"

func TestLintSnippetWithBehavior(t *testing.T) {
	src := baseHead + "CONTRACT\n  R1: add\nSNIPPET s\n  while True:\n"
	doc := parseOK(t, src)
	ds := doc.Lint()
	found := false
	for _, d := range ds {
		if d.Section == "SNIPPET" && d.Severity == SevSuggestion {
			found = true
		}
	}
	if !found {
		t.Errorf("missing snippet/behavior diagnostic: %+v", ds)
	}
}

func TestLintArtifactWithoutSnippet(t *testing.T) {
	src := "INTENT x@1\nKIND program\nFIDELITY artifact\nTARGET py\nCONTRACT\n  R1: add\n"
	doc := parseOK(t, src)
	ds := doc.Lint()
	found := false
	for _, d := range ds {
		if d.Section == "FIDELITY" && d.Severity == SevError {
			found = true
		}
	}
	if !found {
		t.Errorf("expected artifact-without-snippet error, got %+v", ds)
	}
}

func TestLintOpenEmptyDefault(t *testing.T) {
	src := baseHead + "CONTRACT\n  R1: a\nOPEN\n  ?1: 颜色选什么?     default:\n"
	doc := parseOK(t, src)
	ds := doc.Lint()
	found := false
	for _, d := range ds {
		if d.Section == "OPEN" && d.Severity == SevSuggestion {
			found = true
		}
	}
	if !found {
		t.Errorf("expected empty-default suggestion, got %+v", ds)
	}
}

func TestLintClean(t *testing.T) {
	src := "INTENT x@1\nKIND program\nFIDELITY structure\nTARGET py\n" +
		"CONTRACT\n  R1: 支持加法和减法\nACCEPT\n  A1: add(1,2)==3\n  A2: subtract(3,1)==2\n" +
		"DECISIONS\n  D1: 只支持加减     reject: 乘除     due: 简化\nSNIPPET s\n  def f(): pass\nMETA\n  spec: v1.0\n"
	doc := parseOK(t, src)
	ds := doc.Lint()
	if len(ds) != 0 {
		t.Errorf("expected clean lint, got %+v", ds)
	}
}

func TestLintString(t *testing.T) {
	src := baseHead + "CONTRACT\n  R1: a\nSNIPPET s\n  code\n"
	doc := parseOK(t, src)
	s := doc.LintString()
	if !strings.Contains(s, "lint:") {
		t.Errorf("LintString missing header: %s", s)
	}
	if !strings.Contains(s, "建议:") {
		t.Errorf("LintString missing fix suggestion: %s", s)
	}
}

func TestLintPassOnDemoFixture(t *testing.T) {
	doc := parseOK(t, readFixture(t, "demo_calc_archive.intent"))
	if errs := doc.Validate(); len(errs) != 0 {
		t.Fatalf("demo fixture must validate: %v", errs)
	}
	for _, d := range doc.Lint() {
		if d.Severity == SevError {
			t.Errorf("demo fixture should have no structural lint errors, got: %s", d)
		}
	}
}

func TestLintDanglingReference(t *testing.T) {
	src := baseHead + "CONTRACT\n  R1: 输入见 R9，输出按 R1\nACCEPT\n  A1: 见 A9\n  A2: 见 R1\nDECISIONS\n  D1: 见 D9\nOPEN\n  ?1: 见 ?9     default: x\n"
	doc := parseOK(t, src)
	ds := doc.Lint()
	want := map[string]bool{"R9": false, "A9": false, "D9": false, "?9": false}
	for _, d := range ds {
		if d.Severity == SevError {
			for ref := range want {
				if strings.Contains(d.Msg, ref) {
					want[ref] = true
				}
			}
		}
	}
	for ref, found := range want {
		if !found {
			t.Errorf("dangling ref %s not reported; all=%+v", ref, ds)
		}
	}
	// R1 exists, so "输出按 R1" must NOT be flagged.
	for _, d := range ds {
		if strings.Contains(d.Msg, "R1") && strings.Contains(d.Msg, "不存在") {
			t.Errorf("existing R1 should not be dangle: %s", d)
		}
	}
}

func TestLintNoDanglingWhenRefsResolve(t *testing.T) {
	src := baseHead + "CONTRACT\n  R1: a\n  R2: b\nACCEPT\n  A1: 见 R1\nDECISIONS\n  D1: 见 R2     reject: x     due: y\n"
	doc := parseOK(t, src)
	for _, d := range doc.Lint() {
		if d.Severity == SevError {
			t.Errorf("resolved refs flagged: %s", d)
		}
	}
}

func TestLintOpenConverged(t *testing.T) {
	src := baseHead + "CONTRACT\n  R1: 错误时输出 非法输入\nOPEN\n  ?1: 错误消息文案     default: 非法输入\n"
	doc := parseOK(t, src)
	ds := doc.Lint()
	found := false
	for _, d := range ds {
		if d.Section == "OPEN" && d.Severity == SevSuggestion && strings.Contains(d.Msg, "已被 CONTRACT") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected OPEN-converged suggestion, got %+v", ds)
	}
}

func TestLintOpenNotConverged(t *testing.T) {
	src := baseHead + "CONTRACT\n  R1: 输出计算结果\nOPEN\n  ?1: 结果精度保留几位     default: 保留2位小数\n"
	doc := parseOK(t, src)
	for _, d := range doc.Lint() {
		if d.Section == "OPEN" && strings.Contains(d.Msg, "已被 CONTRACT") {
			t.Errorf("unrelated open flagged: %s", d)
		}
	}
}
