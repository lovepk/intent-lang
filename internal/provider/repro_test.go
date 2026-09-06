package provider

import (
	"context"
	"strings"
	"testing"
)

func TestMockRepro(t *testing.T) {
	m := NewMock("mock-a")
	ctx := context.Background()

	archive := ""
	for _, msg := range []string{
		"做一个计算器，支持加法和减法",
		"加上乘法",
	} {
		resp, err := m.Complete(ctx, Request{User: msg, Archive: archive})
		if err != nil {
			t.Fatal(err)
		}
		archive = resp.IntentUpdate
	}

	resp, err := m.Complete(ctx, Request{Archive: archive, User: "重建", Mode: ModeRepro})
	if err != nil {
		t.Fatal(err)
	}
	out := resp.Reply
	for _, want := range []string{"def add(", "def subtract(", "def multiply(", "if __name__"} {
		if !strings.Contains(out, want) {
			t.Errorf("repro output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "def divide(") {
		t.Error("repro output should not contain divide")
	}
}

func TestMockReproDeterministic(t *testing.T) {
	m := NewMock("mock-a")
	ctx := context.Background()
	resp1, err := m.Complete(ctx, Request{Archive: sampleArchive(t), User: "重建", Mode: ModeRepro})
	if err != nil {
		t.Fatal(err)
	}
	resp2, err := m.Complete(ctx, Request{Archive: sampleArchive(t), User: "重建", Mode: ModeRepro})
	if err != nil {
		t.Fatal(err)
	}
	if resp1.Reply != resp2.Reply {
		t.Error("mock repro not deterministic")
	}
}

func TestStripFence(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"```python\nprint(1)\n```", "print(1)"},
		{"```\nprint(1)\n```", "print(1)"},
		{"print(1)", "print(1)"},
		{"```python\nprint(1)", "print(1)"},
	}
	for _, c := range cases {
		if got := StripFence(c.in); got != c.want {
			t.Errorf("StripFence(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
