package provider

import (
	"context"
	"strings"
	"testing"

	"intent-lang/internal/il"
)

func sampleArchive(t *testing.T) string {
	t.Helper()
	m := NewMock("mock-a")
	ctx := context.Background()
	archive := ""
	for _, msg := range []string{
		"做一个计算器，支持加法和减法",
		"加上乘法",
		"加上除法",
	} {
		resp, err := m.Complete(ctx, Request{User: msg, Archive: archive})
		if err != nil {
			t.Fatal(err)
		}
		archive = resp.IntentUpdate
	}
	return archive
}

func TestMockDeterministic(t *testing.T) {
	m := NewMock("mock-a")
	ctx := context.Background()

	run := func() []string {
		var b strings.Builder
		archive := ""
		for _, msg := range []string{
			"做一个计算器，支持加法和减法",
			"加上乘法",
			"加上除法",
			"改成GUI界面，用tkinter",
		} {
			resp, err := m.Complete(ctx, Request{User: msg, Archive: archive})
			if err != nil {
				t.Fatalf("complete(%q): %v", msg, err)
			}
			archive = resp.IntentUpdate
			b.WriteString(resp.Reply + "\n")
			b.WriteString("===\n")
			b.WriteString(archive)
			b.WriteString("\n@@\n")
		}
		return []string{b.String(), archive}
	}

	a := run()
	b := run()
	if a[0] != b[0] {
		t.Error("mock output not deterministic across runs")
	}
	if a[1] != b[1] {
		t.Error("final archive not deterministic")
	}
}

func TestMockScenarioBuildsArchive(t *testing.T) {
	m := NewMock("mock-a")
	ctx := context.Background()
	archive := ""

	steps := []struct {
		msg      string
		wantText string
	}{
		{"做一个计算器，支持加法和减法", "R1: add(a,b) -> a+b"},
		{"加上乘法", "R3: multiply(a,b) -> a*b"},
		{"加上除法", "R4: divide(a,b) -> a/b"},
		{"改成GUI界面，用tkinter", "tkinter"},
	}

	for i, s := range steps {
		resp, err := m.Complete(ctx, Request{User: s.msg, Archive: archive})
		if err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		if resp.IntentUpdate == "" {
			t.Fatalf("step %d: expected intent update, got none", i)
		}
		if !strings.Contains(resp.IntentUpdate, s.wantText) {
			t.Errorf("step %d: archive missing %q:\n%s", i, s.wantText, resp.IntentUpdate)
		}
		if strings.Contains(s.msg, "GUI") && !strings.Contains(resp.IntentUpdate, "FIDELITY artifact") {
			t.Errorf("step %d: GUI should raise fidelity to artifact", i)
		}
		doc, err := il.Parse(resp.IntentUpdate)
		if err != nil {
			t.Fatalf("step %d: parse: %v", i, err)
		}
		if errs := doc.Validate(); len(errs) != 0 {
			t.Errorf("step %d: validate: %v", i, errs)
		}
		archive = resp.IntentUpdate
	}

	if !strings.Contains(archive, "multiply(a,b)") {
		t.Error("final archive lacks multiply")
	}
	if !strings.Contains(archive, "divide(a,b)") {
		t.Error("final archive lacks divide")
	}
}

func TestMockNoChangeOnUnrelatedMessage(t *testing.T) {
	m := NewMock("mock-a")
	ctx := context.Background()

	resp, err := m.Complete(ctx, Request{User: "做一个计算器，支持加法和减法", Archive: ""})
	if err != nil {
		t.Fatal(err)
	}
	archive := resp.IntentUpdate

	resp, err = m.Complete(ctx, Request{User: "今天天气怎么样", Archive: archive})
	if err != nil {
		t.Fatal(err)
	}
	if resp.IntentUpdate != "" {
		t.Errorf("unrelated message should not change archive, got update:\n%s", resp.IntentUpdate)
	}
}

func TestMockNoDuplicateOps(t *testing.T) {
	m := NewMock("mock-a")
	ctx := context.Background()

	archive := ""
	for _, msg := range []string{
		"做一个计算器，支持加法和减法",
		"加上乘法",
		"再加上一个乘法功能",
		"还要支持乘法",
	} {
		resp, err := m.Complete(ctx, Request{User: msg, Archive: archive})
		if err != nil {
			t.Fatalf("complete(%q): %v", msg, err)
		}
		if resp.IntentUpdate != "" {
			archive = resp.IntentUpdate
		}
	}

	count := strings.Count(archive, "multiply(a,b)")
	if count != 1 {
		t.Errorf("expected multiply once, got %d:\n%s", count, archive)
	}
}
