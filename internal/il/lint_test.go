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
	if len(ds) == 0 {
		t.Fatal("expected SNIPPET+FIDELITY=behavior suggestion")
	}
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

func TestNegatedTokensProbe(t *testing.T) {
	line := "R1: 只支持加减乘法，不做除法"
	t.Logf("negatedTokens=%q", negatedTokens(line))
	t.Logf("operatorTokens(除法)=%v", operatorTokens("除法"))
	t.Logf("operatorTokens(divide)=%v", operatorTokens("divide(1,2) works"))
}

func TestLintAcceptUsesNegatedFeature(t *testing.T) {
	src := baseHead + "CONTRACT\n  R1: 只支持加减乘法，不做除法\nACCEPT\n  A1: divide(1,2) works\n"
	doc := parseOK(t, src)
	ds := doc.Lint()
	t.Logf("diags: %+v", ds)
	found := false
	for _, d := range ds {
		if d.Section == "ACCEPT" && d.Severity == SevError && strings.Contains(d.Msg, "divide") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected accept/negation error, got %+v", ds)
	}
}

func TestLintRejectedFeatureStillInContract(t *testing.T) {
	src := baseHead + "CONTRACT\n  R1: 支持除法\n  R2: 支持加法\nDECISIONS\n  D1: 不做除法     reject: 除法     due: 简化\n"
	doc := parseOK(t, src)
	ds := doc.Lint()
	found := false
	for _, d := range ds {
		if d.Section == "CONTRACT" && d.Severity == SevError && strings.Contains(d.Msg, "divide") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected reject/contract error, got %+v", ds)
	}
}

func TestLintClean(t *testing.T) {
	src := "INTENT x@1\nKIND program\nFIDELITY structure\nTARGET py\n" +
		"CONTRACT\n  R1: 支持加法和减法\nACCEPT\n  A1: add(1,2)==3\n  A2: subtract(3,1)==2\n" +
		"DECISIONS\n  D1: 只支持加减     reject: 乘除     due: 简化\nSNIPPET s\n  def f(): pass\n"
	doc := parseOK(t, src)
	ds := doc.Lint()
	if len(ds) != 0 {
		t.Errorf("expected clean lint, got %+v", ds)
	}
}

func TestLintString(t *testing.T) {
	src := baseHead + "CONTRACT\n  R1: 不做除法\nACCEPT\n  A1: divide ok\n"
	doc := parseOK(t, src)
	s := doc.LintString()
	if !strings.Contains(s, "lint:") {
		t.Errorf("LintString missing header: %s", s)
	}
	if !strings.Contains(s, "建议:") {
		t.Errorf("LintString missing fix suggestion: %s", s)
	}
}
