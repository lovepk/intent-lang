package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFmtIdempotentAndCanonical(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "x.il")
	messy := "CONTRACT\n  R1: a\nINTENT x@1\nKIND program\nFIDELITY behavior\nTARGET py\n"
	if err := os.WriteFile(file, []byte(messy), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fmtCmd([]string{file, "--write"}); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := fmtCmd([]string{file, "--write"}); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(file)
	if string(first) != string(second) {
		t.Errorf("fmt not idempotent:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	if strings.Index(string(first), "INTENT x@1") > strings.Index(string(first), "CONTRACT") {
		t.Errorf("header not ordered before CONTRACT:\n%s", first)
	}
}
