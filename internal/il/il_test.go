package il

import (
	"os"
	"strings"
	"testing"
)

func loadTestDoc(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../testdata/calculator.intent")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestParseValidArchive(t *testing.T) {
	doc, err := Parse(loadTestDoc(t))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Header.Intent != "calculator@1.0.0" {
		t.Errorf("intent = %q", doc.Header.Intent)
	}
	if doc.Header.Fidelity != "artifact" {
		t.Errorf("fidelity = %q", doc.Header.Fidelity)
	}
	if sec := doc.Section("CONTRACT"); sec == nil || len(sec.Lines) == 0 {
		t.Error("CONTRACT missing or empty")
	}
	if errs := doc.Validate(); len(errs) != 0 {
		t.Errorf("validate: %v", errs)
	}
}

func TestParseRealLLMVerifiedArchive(t *testing.T) {
	data, err := os.ReadFile("../../testdata/calculator_llm_verified.intent")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(string(data))
	if err != nil {
		t.Fatalf("parse real LLM archive: %v", err)
	}
	if errs := doc.Validate(); len(errs) != 0 {
		t.Errorf("real LLM archive must pass validation, got: %v", errs)
	}
	if doc.Header.Fidelity == "" {
		t.Error("real LLM archive missing FIDELITY")
	}
	if sec := doc.Section("DECISIONS"); sec == nil || len(sec.Lines) == 0 {
		t.Error("real LLM archive should carry DECISIONS")
	}
}

func TestCanonicalRoundTrip(t *testing.T) {
	src := loadTestDoc(t)
	doc, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	out := doc.Canonical()
	doc2, err := Parse(out)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if out != doc2.Canonical() {
		t.Errorf("canonical not stable\nfirst:\n%s\nsecond:\n%s", out, doc2.Canonical())
	}
	if !strings.Contains(out, "eval_core") {
		t.Errorf("SNIPPET label lost:\n%s", out)
	}
}

func TestValidateMissingHeaders(t *testing.T) {
	doc, err := Parse("INTENT x@1.0.0\nKIND program\nCONTRACT\n  R1: hi\n")
	if err != nil {
		t.Fatal(err)
	}
	errs := doc.Validate()
	found := map[string]bool{}
	for _, e := range errs {
		found[e.Error()] = true
	}
	if !found["missing header FIDELITY"] {
		t.Errorf("expected missing FIDELITY, got %v", errs)
	}
	if !found["missing header TARGET"] {
		t.Errorf("expected missing TARGET, got %v", errs)
	}
}

func TestValidateBadFidelity(t *testing.T) {
	src := loadTestDoc(t)
	src = strings.Replace(src, "FIDELITY artifact", "FIDELITY exact", 1)
	doc, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if errs := doc.Validate(); len(errs) == 0 {
		t.Error("expected invalid fidelity error")
	}
}

func TestValidateNonIncreasingContract(t *testing.T) {
	doc, err := Parse("INTENT x@1\nKIND program\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: a\n  R3: b\n  R2: c\n")
	if err != nil {
		t.Fatal(err)
	}
	errs := doc.Validate()
	if len(errs) == 0 {
		t.Error("expected non-increasing R id error")
	}
	for _, e := range errs {
		if strings.Contains(e.Error(), "not increasing") {
			return
		}
	}
	t.Errorf("expected not-increasing error, got %v", errs)
}

func TestValidateDuplicateContractBody(t *testing.T) {
	doc, err := Parse("INTENT x@1\nKIND program\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: add\n  R2: add\n")
	if err != nil {
		t.Fatal(err)
	}
	errs := doc.Validate()
	if len(errs) == 0 {
		t.Error("expected duplicate body error")
	}
	for _, e := range errs {
		if strings.Contains(e.Error(), "duplicate body") {
			return
		}
	}
	t.Errorf("expected duplicate body error, got %v", errs)
}

func TestValidateOpenMissingDefault(t *testing.T) {
	doc, err := Parse("INTENT x@1\nKIND program\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: a\nOPEN\n  ?1: not decided yet\n")
	if err != nil {
		t.Fatal(err)
	}
	errs := doc.Validate()
	if len(errs) == 0 {
		t.Error("expected missing default error")
	}
	for _, e := range errs {
		if strings.Contains(e.Error(), "missing default") {
			return
		}
	}
	t.Errorf("expected missing default error, got %v", errs)
}

func TestValidateEmptySection(t *testing.T) {
	doc, err := Parse("INTENT x@1\nKIND program\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: a\nACCEPT\n")
	if err != nil {
		t.Fatal(err)
	}
	errs := doc.Validate()
	if len(errs) == 0 {
		t.Error("expected empty section error")
	}
	for _, e := range errs {
		if strings.Contains(e.Error(), "empty section") {
			return
		}
	}
	t.Errorf("expected empty section error, got %v", errs)
}

func TestParseContentBeforeSection(t *testing.T) {
	if _, err := Parse("garbage text\nINTENT x@1\n"); err == nil {
		t.Error("expected error for content before header")
	}
}

func TestMetaStripAndAttach(t *testing.T) {
	doc, err := Parse("INTENT x@1\nKIND program\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: a\nMETA\n  spec: v1.0\n  commits: 3\n")
	if err != nil {
		t.Fatal(err)
	}
	noMeta := doc.StripMeta()
	if noMeta.Section("META") != nil {
		t.Error("StripMeta should remove META section")
	}
	with := noMeta.WithMeta([]string{"spec: v1.0", "commits: 4"})
	meta := with.Section("META")
	if meta == nil || len(meta.Lines) != 2 {
		t.Error("WithMeta should append META lines")
	}
	if with.Canonical() == noMeta.Canonical() {
		t.Error("canonical should differ after WithMeta")
	}
	// round-trip canonical must keep META last and parseable
	re, err := Parse(with.Canonical())
	if err != nil {
		t.Fatal(err)
	}
	if re.Section("META") == nil {
		t.Error("re-parsed doc lost META")
	}
}

func TestNextID(t *testing.T) {
	doc, err := Parse("INTENT x@1\nKIND program\nFIDELITY behavior\nTARGET py\nCONTRACT\n  R1: a\n  R2: b\nACCEPT\n  A1: ok\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.NextID("CONTRACT"); got != "R3" {
		t.Errorf("NextID CONTRACT = %s, want R3", got)
	}
	if got := doc.NextID("ACCEPT"); got != "A2" {
		t.Errorf("NextID ACCEPT = %s, want A2", got)
	}
	if got := doc.NextID("DECISIONS"); got != "D1" {
		t.Errorf("NextID DECISIONS = %s, want D1", got)
	}
	if got := doc.NextID("OPEN"); got != "?1" {
		t.Errorf("NextID OPEN = %s, want ?1", got)
	}
}
