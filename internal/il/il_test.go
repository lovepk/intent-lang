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
	for _, file := range []string{
		"../../testdata/calculator_llm_verified.intent",
		"../../testdata/demo_calc_archive.intent",
		"../../testdata/compliance_password_archive.intent",
	} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		doc, err := Parse(string(data))
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		if errs := doc.Validate(); len(errs) != 0 {
			t.Errorf("%s must pass validation, got: %v", file, errs)
		}
		if doc.Header.Fidelity == "" {
			t.Errorf("%s missing FIDELITY", file)
		}
	}
}

func TestParseRealLLMDecisionsPresent(t *testing.T) {
	data, err := os.ReadFile("../../testdata/calculator_llm_verified.intent")
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := Parse(string(data))
	if sec := doc.Section("DECISIONS"); sec == nil || len(sec.Lines) == 0 {
		t.Error("real LLM archive should carry DECISIONS")
	}
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../../testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestParseSnippetKeywordLikeContent(t *testing.T) {
	// Indented lines inside SNIPPET that look like section names must NOT
	// start a new section (regression for the flush-left section detection).
	src := "INTENT x@1\nKIND program\nFIDELITY structure\nTARGET any\nCONTRACT\n  R1: a\n" +
		"SNIPPET core\n  META 42\n  OPEN file\n  CONTRACT something\nACCEPT\n  A1: ok\n"
	doc, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	snip := doc.Section("SNIPPET")
	if snip == nil || len(snip.Lines) != 3 {
		t.Fatalf("SNIPPET should hold 3 indented lines, got %+v", snip)
	}
	if doc.Section("ACCEPT") == nil {
		t.Error("ACCEPT section was swallowed")
	}
}

func TestParseTopLevelKeywordLineIsNotSection(t *testing.T) {
	// A flush-left non-section-name line still belongs to the current section.
	src := "INTENT x@1\nKIND program\nFIDELITY structure\nTARGET any\nCONTRACT\n  R1: a\n" +
		"SNIPPET core\nwhile True:\n  pass\nACCEPT\n  A1: ok\n"
	doc, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	snip := doc.Section("SNIPPET")
	if snip == nil || len(snip.Lines) != 2 {
		t.Fatalf("flush-left code line should stay in SNIPPET, got %+v", snip)
	}
	if doc.Section("ACCEPT") == nil {
		t.Error("ACCEPT swallowed")
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
