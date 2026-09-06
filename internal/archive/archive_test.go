package archive

import (
	"strings"
	"testing"

	"intent-lang/internal/il"
)

func testDoc(contract ...string) string {
	var b strings.Builder
	b.WriteString("INTENT calc@1.0.0\nKIND program\nFIDELITY behavior\nTARGET py\nCONTRACT\n")
	for _, c := range contract {
		b.WriteString("  " + c + "\n")
	}
	b.WriteString("ACCEPT\n  A1: ok\n")
	return b.String()
}

func mustParse(t *testing.T, s string) string {
	t.Helper()
	doc, err := il.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return doc.Canonical()
}

func TestAppendAndHead(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	v1 := mustParse(t, testDoc("R1: add"))
	if _, err := store.Append("init", "做加减", "", v1, "hi", nil); err != nil {
		t.Fatal(err)
	}
	v2 := mustParse(t, testDoc("R1: add", "R2: multiply"))
	if _, err := store.Append("feat", "加乘法", v1, v2, "hi2", nil); err != nil {
		t.Fatal(err)
	}
	head, err := store.HeadID()
	if err != nil {
		t.Fatal(err)
	}
	latest, err := store.Latest()
	if err != nil {
		t.Fatal(err)
	}
	if latest.ID != head {
		t.Errorf("head=%s latest=%s", head, latest.ID)
	}
	if latest.Parent == "" {
		t.Error("expected parent link")
	}
	log, err := store.Log()
	if err != nil {
		t.Fatal(err)
	}
	if len(log) != 2 {
		t.Errorf("log len=%d, want 2", len(log))
	}
}

func TestPersistAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	s1, _ := Open(dir)
	v1 := mustParse(t, testDoc("R1: add"))
	if _, err := s1.Append("init", "做加减", "", v1, "hi", nil); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	latest, err := s2.Latest()
	if err != nil {
		t.Fatal(err)
	}
	if latest == nil || latest.Archive != v1 {
		t.Errorf("reopen lost latest: %+v", latest)
	}
}

func TestSummarize(t *testing.T) {
	before := mustParse(t, testDoc("R1: add"))
	after := mustParse(t, testDoc("R1: add", "R2: multiply"))
	msg := Summarize(before, after)
	if !strings.Contains(msg, "multiply") {
		t.Errorf("summary missing added feature: %q", msg)
	}
	if !strings.Contains(msg, "+") {
		t.Errorf("summary should mark addition: %q", msg)
	}
}

func TestSetHeadRollback(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	v1 := mustParse(t, testDoc("R1: add"))
	c1, err := store.Append("init", "a", "", v1, "hi", []string{"spec: v2.0", "commits: 1"})
	if err != nil {
		t.Fatal(err)
	}
	v2 := mustParse(t, testDoc("R1: add", "R2: multiply"))
	c2, err := store.Append("feat", "b", v1, v2, "hi", []string{"spec: v2.0", "commits: 2"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetHead(c1.ID); err != nil {
		t.Fatal(err)
	}
	latest, err := store.Latest()
	if err != nil {
		t.Fatal(err)
	}
	if latest.ID != c1.ID {
		t.Errorf("after rollback head=%s want %s", latest.ID, c1.ID)
	}
	if !strings.Contains(latest.Archive, "R1: add") || strings.Contains(latest.Archive, "R2: multiply") {
		t.Errorf("after rollback archive should hold v1 only, got:\n%s", latest.Archive)
	}
	// c2 must remain readable
	got, err := store.Get(c2.ID)
	if err != nil || got == nil {
		t.Error("later commit should still be retrievable after rollback")
	}
}

func TestOpenArchiveIsolation(t *testing.T) {
	dir := t.TempDir()
	common, _ := OpenArchive(dir, "common")
	app, _ := OpenArchive(dir, "app")
	cv := mustParse(t, testDoc("R1: common rule"))
	if _, err := common.Append("init", "建 common", "", cv, "ok", []string{"spec: v2.0", "commits: 1"}); err != nil {
		t.Fatal(err)
	}
	av := mustParse(t, testDoc("R1: app rule"))
	if _, err := app.Append("init", "建 app", "", av, "ok", nil); err != nil {
		t.Fatal(err)
	}
	cl, _ := common.Log()
	al, _ := app.Log()
	if len(cl) != 1 || len(al) != 1 {
		t.Fatalf("isolation broken: common=%d app=%d", len(cl), len(al))
	}
	def, _ := OpenArchive(dir, "main")
	if dl, _ := def.Log(); len(dl) != 0 {
		t.Error("default archive should be empty in fresh repo")
	}
	names, _ := app.ListArchives()
	found := false
	for _, n := range names {
		if n == "common" || n == "app" {
			found = true
		}
	}
	if !found {
		t.Errorf("ListArchives missing named archives: %v", names)
	}
}
