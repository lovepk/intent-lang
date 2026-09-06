package il

import (
	"fmt"
	"strings"
)

// Severity is how a lint finding should be treated.
type Severity string

const (
	// SevSuggestion flags a likely oversight worth reviewing.
	SevSuggestion Severity = "suggestion"
	// SevError flags an internal contradiction that will break reproduction.
	SevError Severity = "error"
)

// Diagnostic is a human-readable consistency finding.
type Diagnostic struct {
	Severity Severity
	Section  string
	ID       string
	Msg      string
	Fix      string
}

func (d Diagnostic) String() string {
	where := d.Section
	if d.ID != "" {
		where += " " + d.ID
	}
	line := fmt.Sprintf("[%s] %s: %s", d.Severity, where, d.Msg)
	if d.Fix != "" {
		line += "  建议: " + d.Fix
	}
	return line
}

// Lint performs structural consistency checks that are deterministic and
// language-general (no domain keyword tables). Deeper semantic contradictions
// (e.g. ACCEPT conflicts with a CONTRACT negation, DECISIONS reject vs
// CONTRACT) are checked by the LLM-based linter (see LintPrompt) because they
// require understanding, not keyword matching.
func (d *Doc) Lint() []Diagnostic {
	var out []Diagnostic

	// SNIPPET present but FIDELITY=behavior: locking form without claiming it.
	if d.Section("SNIPPET") != nil && d.Header.Fidelity == "behavior" {
		out = append(out, Diagnostic{
			Severity: SevSuggestion,
			Section:  "SNIPPET",
			Msg:      "档案包含 SNIPPET（点名锁定形态），但 FIDELITY=behavior 只承诺行为一致",
			Fix:      "若这些片段确实要求逐字复现，把 FIDELITY 升为 structure 或 artifact；否则删除未点名的 SNIPPET",
		})
	}

	// META.spec must match current spec version when present.
	if meta := d.MetaLines(); meta != nil {
		for _, line := range meta {
			if strings.HasPrefix(strings.TrimSpace(line), "spec:") {
				val := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "spec:"))
				if val != SpecVersion {
					out = append(out, Diagnostic{
						Severity: SevSuggestion,
						Section:  "META",
						Msg:      fmt.Sprintf("档案声明 spec=%s，当前规范版本是 %s", val, SpecVersion),
						Fix:      "用 il.MetaSpecLine() 重建 META（或确认有意使用旧规范）",
					})
				}
			}
		}
	}

	// FIDELITY=artifact requires at least one SNIPPET (form lock needs original).
	if d.Header.Fidelity == "artifact" && d.Section("SNIPPET") == nil {
		out = append(out, Diagnostic{
			Severity: SevError,
			Section:  "FIDELITY",
			Msg:      "FIDELITY=artifact 要求形态一致，但档案没有 SNIPPET 黄金代码",
			Fix:      "补上要求逐字复现的 SNIPPET，或把 FIDELITY 降为 structure/behavior",
		})
	}

	// OPEN items with empty default value text.
	if sec := d.Section("OPEN"); sec != nil {
		for _, line := range sec.Lines {
			body := entryBody(line)
			if i := strings.Index(body, "default:"); i >= 0 {
				def := strings.TrimSpace(strings.TrimPrefix(body[i:], "default:"))
				if def == "" {
					out = append(out, Diagnostic{
						Severity: SevSuggestion,
						Section:  "OPEN",
						Msg:      fmt.Sprintf("default 为空（%s）", strings.TrimSpace(body)),
						Fix:      "给该开放项一个明确的默认处理",
					})
				}
			}
		}
	}

	return out
}

// LintString renders diagnostics as human-readable lines.
func (d *Doc) LintString() string {
	ds := d.Lint()
	if len(ds) == 0 {
		return "lint: 通过，未发现一致性矛盾\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "lint: 发现 %d 处（%d error / %d suggestion）\n",
		len(ds), countSev(ds, SevError), countSev(ds, SevSuggestion))
	for _, di := range ds {
		b.WriteString("  " + di.String() + "\n")
	}
	return b.String()
}

func countSev(ds []Diagnostic, sev Severity) int {
	n := 0
	for _, d := range ds {
		if d.Severity == sev {
			n++
		}
	}
	return n
}
