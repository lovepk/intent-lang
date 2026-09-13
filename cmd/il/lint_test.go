package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLintFailOnError(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bad.il")
	// FIDELITY=artifact without SNIPPET is an error-severity diagnostic.
	bad := "INTENT x@1\nKIND program\nFIDELITY artifact\nTARGET py\nCONTRACT\n  R1: a\n"
	if err := os.WriteFile(file, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := lint([]string{"--archive", file, "--fail-on", "error"}); err == nil {
		t.Error("expected non-nil error for --fail-on error")
	}
	if err := lint([]string{"--archive", file}); err != nil {
		t.Errorf("lint without --fail-on should not error: %v", err)
	}
}

func TestLintFailOnCleanArchive(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "ok.il")
	ok := "INTENT x@1\nKIND program\nFIDELITY behavior\nTARGET python\nCONTRACT\n  R1: a\n  R2: b\nACCEPT\n  A1: ok\n"
	if err := os.WriteFile(file, []byte(ok), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := lint([]string{"--archive", file, "--fail-on", "error"}); err != nil {
		t.Errorf("clean archive should pass --fail-on error: %v", err)
	}
}
