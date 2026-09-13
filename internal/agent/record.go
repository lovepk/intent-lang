package agent

import (
	"context"
	"fmt"
	"strings"

	"intent-lang/internal/il"
	"intent-lang/internal/provider"
)

// L1Problems runs the deterministic (L1) consistency checks on an archive
// draft: structural validity plus deterministic internal-contradiction lint.
// An empty result means the draft is valid and self-consistent, so the L2
// recorder is not needed. These checks are pure computation — they never
// consult an LLM and never reject; they only decide whether L2 must run.
func L1Problems(text string) []string {
	doc, err := il.Parse(text)
	if err != nil {
		return []string{"无法解析为档案：" + err.Error()}
	}
	var out []string
	for _, e := range doc.Validate() {
		out = append(out, "结构："+e.Error())
	}
	for _, d := range doc.Lint() {
		if d.Severity == il.SevError {
			out = append(out, "矛盾："+d.String())
		}
	}
	return out
}

// NormalizeArchive is the L2 role: the same LLM acting as a record keeper that
// turns a draft into a valid, internally consistent archive without changing
// any fact. It is invoked only when L1 finds a problem.
func NormalizeArchive(ctx context.Context, p provider.Provider, draft, previous string, problems []string) (string, error) {
	resp, err := p.Complete(ctx, provider.Request{
		System:  il.NormalizePrompt,
		Archive: previous,
		User:    fmt.Sprintf("【待整理的草稿档案】\n```\n%s\n```\n\n【确定性检查发现的问题】\n%s", draft, strings.Join(problems, "\n")),
		Mode:    provider.ModeNormalize,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(provider.StripFence(resp.Reply)), nil
}

// RecordArchive applies the layered model to a generator draft and returns the
// text that should be recorded, plus whether L2 ran and succeeded. It never
// rejects: if L2 cannot produce a valid archive, the draft is recorded as-is.
func RecordArchive(ctx context.Context, p provider.Provider, draft, previous string) (recorded string, normalized bool) {
	problems := L1Problems(draft)
	if len(problems) == 0 {
		return draft, false
	}
	fixed, err := NormalizeArchive(ctx, p, draft, previous, problems)
	if err != nil || strings.TrimSpace(fixed) == "" {
		return draft, false
	}
	if len(L1Problems(fixed)) != 0 {
		return draft, false
	}
	return fixed, true
}
