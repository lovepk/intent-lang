package intentlang

import (
	"context"
	"strings"
	"testing"
)

type fakeModel struct {
	reply        string
	intentUpdate string
}

func (f fakeModel) Name() string { return "fake" }

func (f fakeModel) Complete(_ context.Context, req Request) (Response, error) {
	if req.Mode == ModeRepro {
		return Response{Reply: "print('hi')"}, nil
	}
	return Response{Reply: f.reply, IntentUpdate: f.intentUpdate}, nil
}

const sample = "INTENT x@1\nKIND program\nFIDELITY behavior\nTARGET python\nCONTRACT\n  R1: a\nACCEPT\n  A1: ok\n"

func TestParseLintDiff(t *testing.T) {
	doc, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Header().Intent != "x@1" || doc.Header().Fidelity != "behavior" {
		t.Errorf("header = %+v", doc.Header())
	}
	if errs := doc.Validate(); len(errs) != 0 {
		t.Errorf("unexpected validate errors: %v", errs)
	}
	if ids := doc.ContractIDs(); len(ids) != 1 || ids[0] != "R1" {
		t.Errorf("contract ids = %v", ids)
	}
	if !strings.Contains(doc.Canonical(), "R1: a") {
		t.Error("canonical missing R1")
	}

	after := "INTENT x@1\nKIND program\nFIDELITY behavior\nTARGET python\nCONTRACT\n  R1: a\n  R2: b\nACCEPT\n  A1: ok\n"
	c := CompareEntries(sample, after)
	if len(c.Added) != 1 || c.Added[0] != "R2" {
		t.Errorf("diff added = %v", c.Added)
	}
}

func TestRecord(t *testing.T) {
	a := New(fakeModel{reply: "好的", intentUpdate: sample})
	turn, err := a.Record(context.Background(), "", "做一个 x")
	if err != nil {
		t.Fatal(err)
	}
	if !turn.Changed || turn.Reply != "好的" || !strings.Contains(turn.Archive, "R1: a") {
		t.Errorf("turn = %+v", turn)
	}
	if len(turn.Diff.Added) == 0 {
		t.Error("expected added units in diff")
	}
}

func TestRecordUnchanged(t *testing.T) {
	a := New(fakeModel{reply: "只是聊天"})
	turn, err := a.Record(context.Background(), sample, "你好")
	if err != nil {
		t.Fatal(err)
	}
	if turn.Changed {
		t.Error("expected unchanged turn")
	}
}

func TestReproduce(t *testing.T) {
	a := New(fakeModel{})
	out, err := a.Reproduce(context.Background(), sample)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "print('hi')") {
		t.Errorf("reproduce = %q", out)
	}
}

func TestStoreCommitLog(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.Commit("init", "需求", "", sample, "reply", BuildMetaLines(1, "", nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.ID == "" || !strings.Contains(c.Archive, "R1: a") {
		t.Errorf("commit = %+v", c)
	}
	latest, err := s.Latest()
	if err != nil || latest == nil || latest.ID != c.ID {
		t.Fatalf("latest = %+v, err=%v", latest, err)
	}
	log, err := s.Log()
	if err != nil || len(log) != 1 {
		t.Fatalf("log = %v, err=%v", log, err)
	}
	created, _ := ParseMetaCarry(c.Archive)
	if created == "" {
		t.Error("expected created timestamp in META")
	}
}
