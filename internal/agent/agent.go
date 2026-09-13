package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"intent-lang/internal/il"
	"intent-lang/internal/provider"
)

// Agent is the model-driven half of IL: it runs the dual-channel turn, the
// reproduction, the acceptance and the LLM lint. All archive storage stays in
// the archive package; the Agent never persists on its own.
type Agent struct {
	Model provider.Provider
}

// Turn is the outcome of one dual-channel record turn.
type Turn struct {
	Reply      string
	Archive    string // full archive text after the update (no META)
	Changed    bool   // false when the message produced no archive change
	Normalized bool   // true when the L2 recorder rewrote a bad draft
	Diff       il.Diff
}

// Record runs one dual-channel turn: the model answers the user and, in the
// same request, updates the archive. It applies the layered model (L1
// deterministic, L2 recorder only when L1 finds a problem) and never rejects.
func (a *Agent) Record(ctx context.Context, beforeRaw, userMsg string) (Turn, error) {
	beforeDoc := NoMeta(beforeRaw)
	resp, err := a.Model.Complete(ctx, provider.Request{
		System:  il.SpecPrompt,
		Archive: beforeDoc,
		User:    userMsg,
	})
	if err != nil {
		return Turn{}, err
	}
	if strings.TrimSpace(resp.IntentUpdate) == "" {
		return Turn{Reply: resp.Reply}, nil
	}
	after, normalized := RecordArchive(ctx, a.Model, resp.IntentUpdate, beforeDoc)
	return Turn{
		Reply:      resp.Reply,
		Archive:    after,
		Changed:    true,
		Normalized: normalized,
		Diff:       il.CompareEntries(beforeRaw, after),
	}, nil
}

// Reproduce rebuilds the product from a self-contained archive (references
// already expanded). It validates the archive first.
func (a *Agent) Reproduce(ctx context.Context, resolvedArchive string) (string, error) {
	doc, err := il.Parse(resolvedArchive)
	if err != nil {
		return "", fmt.Errorf("archive: %w", err)
	}
	if errs := doc.Validate(); len(errs) != 0 {
		return "", fmt.Errorf("archive invalid: %v", errs)
	}
	resp, err := a.Model.Complete(ctx, provider.Request{
		System:  il.ReproPrompt,
		Archive: doc.Canonical(),
		User:    "请据此档案重建产物。",
		Mode:    provider.ModeRepro,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Reply), nil
}

// Accept generates an acceptance script from the archive's ACCEPT cases and
// runs it against the artifact. It returns the generated script and the
// harness output. The harness is Python regardless of artifact language; the
// interpreter can be overridden via IL_PYTHON.
func (a *Agent) Accept(ctx context.Context, resolvedArchive, artifactPath string) (script, output string, err error) {
	absArtifact, err := filepath.Abs(artifactPath)
	if err != nil {
		return "", "", err
	}
	resp, err := a.Model.Complete(ctx, provider.Request{
		System:  il.AcceptTestPrompt,
		Archive: resolvedArchive,
		User:    fmt.Sprintf("产物路径: %s\n请据此生成验收脚本。", absArtifact),
		Mode:    provider.ModeRepro,
	})
	if err != nil {
		return "", "", err
	}
	script = provider.StripFence(resp.Reply)
	tmp := filepath.Join(os.TempDir(), "il_accept_test.py")
	if err := os.WriteFile(tmp, []byte(script), 0o644); err != nil {
		return "", "", err
	}
	python := os.Getenv("IL_PYTHON")
	if python == "" {
		python = "python"
	}
	out, _ := exec.Command(python, "-X", "utf8", tmp, absArtifact).CombinedOutput()
	return script, string(out), nil
}

// LintLLM runs the semantic lint layer and returns its findings.
func (a *Agent) LintLLM(ctx context.Context, archive string) ([]provider.Finding, error) {
	resp, err := a.Model.Complete(ctx, provider.Request{
		System:  il.LintPrompt,
		Archive: archive,
		User:    "请复查这份档案的语义一致性。",
		Mode:    provider.ModeLint,
	})
	if err != nil {
		return nil, err
	}
	return resp.Findings, nil
}

// NoMeta parses text and returns its canonical form without the META section.
// Unparseable text is returned unchanged.
func NoMeta(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	doc, err := il.Parse(text)
	if err != nil {
		return text
	}
	return doc.StripMeta().Canonical()
}
