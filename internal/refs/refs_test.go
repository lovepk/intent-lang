package refs

import (
	"strings"
	"testing"
)

func loaderOf(files map[string]string) Loader {
	return func(name string) (string, bool) {
		t, ok := files[name]
		return t, ok
	}
}

const commonIL = "INTENT common@1.2\nKIND program\nFIDELITY behavior\nTARGET any\n" +
	"CONTRACT\n  R1: 通用错误处理\n  R3: 错误时打印 error 退出码 1\n"

func TestResolveSimple(t *testing.T) {
	files := map[string]string{
		"common": commonIL,
	}
	src := "INTENT app@1.0\nKIND program\nFIDELITY behavior\nTARGET py\n" +
		"CONTRACT\n  R1: 输入校验见 <ref: common#R3@1.2>\n  R2: 本地逻辑\n"
	out, err := Resolve(src, loaderOf(files))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "错误时打印 error 退出码 1") {
		t.Errorf("materialized body missing from common:\n%s", out)
	}
	if strings.Contains(out, "<ref:") {
		t.Errorf("resolved output still contains ref marker:\n%s", out)
	}
	if !strings.Contains(out, "来源:") {
		t.Errorf("resolved output missing provenance:\n%s", out)
	}
	// original local text preserved
	if !strings.Contains(out, "本地逻辑") {
		t.Errorf("local text lost:\n%s", out)
	}
	// local prefix kept
	if !strings.Contains(out, "R1: 输入校验见 错误时打印 error 退出码 1") {
		t.Errorf("local prefix + materialized body should coexist:\n%s", out)
	}
}

func TestResolveVersionMismatch(t *testing.T) {
	files := map[string]string{"common": commonIL}
	src := "INTENT app@1\nKIND p\nFIDELITY behavior\nTARGET py\n" +
		"CONTRACT\n  R1: 见 <ref: common#R3@9.9>\n"
	_, err := Resolve(src, loaderOf(files))
	if err == nil || !strings.Contains(err.Error(), "version mismatch") {
		t.Errorf("expected version mismatch, got %v", err)
	}
}

func TestResolveMissingTarget(t *testing.T) {
	src := "INTENT app@1\nKIND p\nFIDELITY behavior\nTARGET py\n" +
		"CONTRACT\n  R1: 见 <ref: nope#R3@1>\n"
	_, err := Resolve(src, loaderOf(filesEmpty()))
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found error, got %v", err)
	}
}

func TestResolveMissingEntry(t *testing.T) {
	files := map[string]string{"common": commonIL}
	src := "INTENT app@1\nKIND p\nFIDELITY behavior\nTARGET py\n" +
		"CONTRACT\n  R1: 见 <ref: common#R9@1.2>\n"
	_, err := Resolve(src, loaderOf(files))
	if err == nil || !strings.Contains(err.Error(), "R9 not found") {
		t.Errorf("expected missing-entry error, got %v", err)
	}
}

func TestResolveCircular(t *testing.T) {
	a := "INTENT a@1\nKIND p\nFIDELITY behavior\nTARGET py\n" +
		"CONTRACT\n  R1: 引 b <ref: b#R1@1>\n"
	b := "INTENT b@1\nKIND p\nFIDELITY behavior\nTARGET py\n" +
		"CONTRACT\n  R1: 引 a <ref: a#R1@1>\n"
	files := map[string]string{"a": a, "b": b}
	_, err := Resolve(a, loaderOf(files))
	if err == nil || !strings.Contains(err.Error(), "circular") {
		t.Errorf("expected circular error, got %v", err)
	}
}

func TestResolveNested(t *testing.T) {
	// app -> lib (R1) ; lib R1 refs base R2
	base := "INTENT base@1\nKIND p\nFIDELITY behavior\nTARGET py\n" +
		"CONTRACT\n  R2: 最底层规则\n"
	lib := "INTENT lib@1\nKIND p\nFIDELITY behavior\nTARGET py\n" +
		"CONTRACT\n  R1: 见 <ref: base#R2@1> 再加本地\n"
	files := map[string]string{"lib": lib, "base": base}
	src := "INTENT app@1\nKIND p\nFIDELITY behavior\nTARGET py\n" +
		"CONTRACT\n  R1: 引用 <ref: lib#R1@1>\n"
	out, err := Resolve(src, loaderOf(files))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "最底层规则") || !strings.Contains(out, "再加本地") {
		t.Errorf("nested resolve incomplete:\n%s", out)
	}
	if strings.Contains(out, "<ref:") {
		t.Errorf("nested output still has refs:\n%s", out)
	}
}

func filesEmpty() map[string]string { return map[string]string{} }
