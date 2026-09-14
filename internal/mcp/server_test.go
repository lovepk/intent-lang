package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lovepk/intent-lang/internal/provider"
)

func rpcReq(t *testing.T, id int, method string, params any) string {
	t.Helper()
	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			t.Fatal(err)
		}
		raw = b
	}
	msg := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if raw != nil {
		msg["params"] = raw
	}
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// call sends a single request and returns the decoded response.
func call(t *testing.T, repo, line string) map[string]json.RawMessage {
	return callModel(t, repo, nil, line)
}

func callModel(t *testing.T, repo string, model provider.Provider, line string) map[string]json.RawMessage {
	t.Helper()
	var out bytes.Buffer
	srv := NewServer(strings.NewReader(line+"\n"), &out, repo, "main")
	if model != nil {
		srv.WithModel(model)
	}
	if err := srv.Serve(); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	dec := json.NewDecoder(&out)
	var resp map[string]json.RawMessage
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("decode response: %v (raw: %s)", err, out.String())
	}
	return resp
}

// fakeModel is an offline provider for exercising the orchestrator tools.
type fakeModel struct {
	reply        string
	intentUpdate string
}

func (f *fakeModel) Name() string { return "fake" }

func (f *fakeModel) Complete(_ context.Context, req provider.Request) (provider.Response, error) {
	switch req.Mode {
	case provider.ModeRepro:
		return provider.Response{Reply: "print('hi')"}, nil
	case provider.ModeLint:
		return provider.Response{Findings: []provider.Finding{
			{Severity: "error", Section: "ACCEPT", ID: "A1", Msg: "与 CONTRACT 冲突", Fix: "删掉 A1"},
		}}, nil
	}
	return provider.Response{Reply: f.reply, IntentUpdate: f.intentUpdate}, nil
}

func result(t *testing.T, resp map[string]json.RawMessage) json.RawMessage {
	t.Helper()
	if e, ok := resp["error"]; ok {
		t.Fatalf("unexpected rpc error: %s", e)
	}
	r, ok := resp["result"]
	if !ok {
		t.Fatal("response has no result")
	}
	return r
}

func toolText(t *testing.T, resp map[string]json.RawMessage) string {
	t.Helper()
	var r struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(result(t, resp), &r); err != nil {
		t.Fatalf("unmarshal tool result: %v", err)
	}
	if len(r.Content) == 0 {
		t.Fatal("tool result has no content")
	}
	return r.Content[0].Text
}

const sampleArchive = "INTENT x@1\nKIND program\nFIDELITY behavior\nTARGET python\nCONTRACT\n  R1: a\n  R2: b\nACCEPT\n  A1: ok\n"

func TestInitialize(t *testing.T) {
	resp := call(t, "", rpcReq(t, 1, "initialize", map[string]any{"protocolVersion": "2024-11-05"}))
	var r struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name string `json:"name"`
		} `json:"serverInfo"`
		Capabilities map[string]any `json:"capabilities"`
		Instructions string         `json:"instructions"`
	}
	if err := json.Unmarshal(result(t, resp), &r); err != nil {
		t.Fatal(err)
	}
	if r.ProtocolVersion != "2024-11-05" || r.ServerInfo.Name != "intent-lang" {
		t.Errorf("initialize result = %+v", r)
	}
	if !strings.Contains(r.Instructions, "BYOM") {
		t.Errorf("instructions missing guidance: %q", r.Instructions)
	}
	for _, cap := range []string{"tools", "resources", "prompts"} {
		if _, ok := r.Capabilities[cap]; !ok {
			t.Errorf("missing capability %q", cap)
		}
	}
}

func TestToolsList(t *testing.T) {
	resp := call(t, "", rpcReq(t, 1, "tools/list", nil))
	var r struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result(t, resp), &r); err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, tl := range r.Tools {
		got[tl.Name] = true
	}
	for _, want := range []string{"il_parse", "il_lint", "il_diff", "il_resolve", "il_read", "il_commit", "il_log"} {
		if !got[want] {
			t.Errorf("tool %q missing from list", want)
		}
	}
}

func TestToolParse(t *testing.T) {
	resp := call(t, "", rpcReq(t, 1, "tools/call", map[string]any{
		"name":      "il_parse",
		"arguments": map[string]any{"text": sampleArchive},
	}))
	text := toolText(t, resp)
	for _, want := range []string{"INTENT x@1", "CONTRACT: 2 行", "ACCEPT: 1 行", "R1: a"} {
		if !strings.Contains(text, want) {
			t.Errorf("il_parse output missing %q:\n%s", want, text)
		}
	}
}

func TestToolLintClean(t *testing.T) {
	resp := call(t, "", rpcReq(t, 1, "tools/call", map[string]any{
		"name":      "il_lint",
		"arguments": map[string]any{"text": sampleArchive},
	}))
	if got := toolText(t, resp); !strings.Contains(got, "通过") {
		t.Errorf("expected clean lint, got: %s", got)
	}
}

func TestToolLintFindsProblem(t *testing.T) {
	bad := "INTENT x@1\nKIND program\nFIDELITY artifact\nTARGET py\nCONTRACT\n  R1: a\n"
	resp := call(t, "", rpcReq(t, 1, "tools/call", map[string]any{
		"name":      "il_lint",
		"arguments": map[string]any{"text": bad},
	}))
	if got := toolText(t, resp); !strings.Contains(got, "SNIPPET") {
		t.Errorf("expected SNIPPET diagnostic, got: %s", got)
	}
}

func TestToolDiff(t *testing.T) {
	after := "INTENT x@1\nKIND program\nFIDELITY behavior\nTARGET python\nCONTRACT\n  R1: a\n  R2: b\n  R3: c\nACCEPT\n  A1: ok\n"
	resp := call(t, "", rpcReq(t, 1, "tools/call", map[string]any{
		"name":      "il_diff",
		"arguments": map[string]any{"before": sampleArchive, "after": after},
	}))
	if got := toolText(t, resp); !strings.Contains(got, "+ R3") {
		t.Errorf("expected added R3, got: %s", got)
	}
}

func TestToolCommitAndLog(t *testing.T) {
	repo := t.TempDir()
	resp := call(t, repo, rpcReq(t, 1, "tools/call", map[string]any{
		"name": "il_commit",
		"arguments": map[string]any{
			"after":    sampleArchive,
			"user_msg": "初始需求",
		},
	}))
	if got := toolText(t, resp); !strings.Contains(got, "commit c-") {
		t.Fatalf("expected a commit id, got: %s", got)
	}

	resp = call(t, repo, rpcReq(t, 2, "tools/call", map[string]any{
		"name":      "il_log",
		"arguments": map[string]any{},
	}))
	if got := toolText(t, resp); !strings.Contains(got, "初始需求") {
		t.Errorf("il_log missing user msg: %s", got)
	}

	resp = call(t, repo, rpcReq(t, 3, "tools/call", map[string]any{
		"name":      "il_read",
		"arguments": map[string]any{},
	}))
	if got := toolText(t, resp); !strings.Contains(got, "INTENT x@1") {
		t.Errorf("il_read missing archive: %s", got)
	}
}

func TestPromptsGet(t *testing.T) {
	resp := call(t, "", rpcReq(t, 1, "prompts/get", map[string]any{"name": "write"}))
	var r struct {
		Messages []struct {
			Role    string `json:"role"`
			Content struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(result(t, resp), &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Messages) == 0 || !strings.Contains(r.Messages[0].Content.Text, "意图档案编译器") {
		t.Errorf("write prompt missing spec text")
	}
}

func TestResourceReadSpec(t *testing.T) {
	resp := call(t, "", rpcReq(t, 1, "resources/read", map[string]any{"uri": "il://spec"}))
	var r struct {
		Contents []struct {
			URI  string `json:"uri"`
			Text string `json:"text"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(result(t, resp), &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Contents) == 0 || !strings.Contains(r.Contents[0].Text, "根本原则") {
		t.Errorf("spec resource missing content")
	}
}

func TestUnknownMethod(t *testing.T) {
	resp := call(t, "", rpcReq(t, 1, "bogus/method", nil))
	if _, ok := resp["error"]; !ok {
		t.Fatal("expected error for unknown method")
	}
}

func TestOrchestratorToolsHiddenWithoutModel(t *testing.T) {
	resp := call(t, "", rpcReq(t, 1, "tools/list", nil))
	if strings.Contains(string(result(t, resp)), "il_record") {
		t.Error("il_record should not be advertised without a model")
	}
}

func TestOrchestratorToolsShownWithModel(t *testing.T) {
	resp := callModel(t, "", &fakeModel{}, rpcReq(t, 1, "tools/list", nil))
	raw := string(result(t, resp))
	for _, want := range []string{"il_record", "il_reproduce", "il_accept"} {
		if !strings.Contains(raw, want) {
			t.Errorf("missing orchestrator tool %q", want)
		}
	}
}

func TestToolRecord(t *testing.T) {
	model := &fakeModel{reply: "好的", intentUpdate: sampleArchive}
	resp := callModel(t, "", model, rpcReq(t, 1, "tools/call", map[string]any{
		"name":      "il_record",
		"arguments": map[string]any{"archive": "", "user_msg": "做一个 x"},
	}))
	got := toolText(t, resp)
	for _, want := range []string{"reply:", "好的", "--- archive ---", "INTENT x@1", "changed:"} {
		if !strings.Contains(got, want) {
			t.Errorf("il_record output missing %q:\n%s", want, got)
		}
	}
}

func TestToolRecordUnchanged(t *testing.T) {
	model := &fakeModel{reply: "只是聊天"}
	resp := callModel(t, "", model, rpcReq(t, 1, "tools/call", map[string]any{
		"name":      "il_record",
		"arguments": map[string]any{"user_msg": "你好"},
	}))
	if got := toolText(t, resp); !strings.Contains(got, "档案未变更") {
		t.Errorf("expected unchanged, got: %s", got)
	}
}

func TestToolReproduce(t *testing.T) {
	resp := callModel(t, "", &fakeModel{}, rpcReq(t, 1, "tools/call", map[string]any{
		"name":      "il_reproduce",
		"arguments": map[string]any{"archive": sampleArchive},
	}))
	if got := toolText(t, resp); !strings.Contains(got, "print('hi')") {
		t.Errorf("il_reproduce output = %q", got)
	}
}

func TestToolAccept(t *testing.T) {
	artifact := filepath.Join(t.TempDir(), "a.py")
	if err := os.WriteFile(artifact, []byte("print('x')"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp := callModel(t, "", &fakeModel{}, rpcReq(t, 1, "tools/call", map[string]any{
		"name":      "il_accept",
		"arguments": map[string]any{"archive": sampleArchive, "artifact": artifact},
	}))
	if got := toolText(t, resp); !strings.Contains(got, "生成的验收脚本") {
		t.Errorf("il_accept output = %q", got)
	}
}

func TestToolVerify(t *testing.T) {
	model := &fakeModel{reply: "预览", intentUpdate: sampleArchive}
	resp := callModel(t, "", model, rpcReq(t, 1, "tools/call", map[string]any{
		"name":      "il_verify",
		"arguments": map[string]any{"archive": sampleArchive, "user_msg": "加上 R2"},
	}))
	got := toolText(t, resp)
	for _, want := range []string{"reply:", "预览", "--- archive ---"} {
		if !strings.Contains(got, want) {
			t.Errorf("il_verify output missing %q:\n%s", want, got)
		}
	}
}

func TestToolLintSemantic(t *testing.T) {
	resp := callModel(t, "", &fakeModel{}, rpcReq(t, 1, "tools/call", map[string]any{
		"name":      "il_lint_semantic",
		"arguments": map[string]any{"archive": sampleArchive},
	}))
	got := toolText(t, resp)
	if !strings.Contains(got, "与 CONTRACT 冲突") || !strings.Contains(got, "ACCEPT A1") {
		t.Errorf("il_lint_semantic output = %q", got)
	}
}
