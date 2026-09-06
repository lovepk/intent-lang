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

// negatedTokens returns operator/feature tokens mentioned right after a
// negation keyword, e.g. "不做除法" -> ["除法", "/"].
func negatedTokens(text string) []string {
	lower := strings.ToLower(text)
	var out []string
	for _, marker := range []string{"不做", "不支持", "不提供", "禁止", "移除", "no ", "reject:"} {
		idx := strings.Index(lower, strings.ToLower(marker))
		if idx < 0 {
			continue
		}
		rest := text[idx+len(marker):]
		if i := strings.IndexAny(rest, ",;，；。\n"); i >= 0 {
			rest = rest[:i]
		}
		rest = strings.TrimSpace(rest)
		if rest != "" {
			out = append(out, rest)
		}
	}
	return out
}

// operatorTokens normalizes operator mentions to a comparable token set.
func operatorTokens(text string) map[string]bool {
	tokens := map[string]bool{}
	reps := []struct{ from, to string }{
		{"除法", "divide"}, {"乘法", "multiply"}, {"减法", "subtract"}, {"加法", "add"},
		{"除以", "divide"}, {"乘以", "multiply"}, {"减去", "subtract"}, {"加上", "add"},
		{"+", "add"}, {"-", "subtract"}, {"*", "multiply"}, {"/", "divide"},
		{"×", "multiply"}, {"÷", "divide"},
		{"divide", "divide"}, {"division", "divide"},
		{"multiply", "multiply"}, {"multiplication", "multiply"},
		{"subtract", "subtract"}, {"subtraction", "subtract"},
		{"add", "add"}, {"addition", "add"},
		{"plus", "add"}, {"minus", "subtract"}, {"times", "multiply"},
		{"addition", "add"}, {"减法运算", "subtract"}, {"加法运算", "add"},
	}
	t := strings.ToLower(text)
	for _, r := range reps {
		if strings.Contains(t, r.from) {
			tokens[r.to] = true
		}
	}
	return tokens
}

func mapContainsAny(m map[string]bool, s []string) bool {
	for _, x := range s {
		toks := operatorTokens(x)
		for t := range toks {
			if m[t] {
				return true
			}
		}
	}
	return false
}

// Lint performs consistency checks beyond structural Validate. It is heuristic:
// it flags contradictions that mechanical rules can catch; final judgment on
// reproduced artifacts still belongs to ACCEPT acceptance.
func (d *Doc) Lint() []Diagnostic {
	var out []Diagnostic

	// 1. SNIPPET present but FIDELITY=behavior: locking form without claiming it.
	if d.Section("SNIPPET") != nil && d.Header.Fidelity == "behavior" {
		out = append(out, Diagnostic{
			Severity: SevSuggestion,
			Section:  "SNIPPET",
			Msg:      "档案包含 SNIPPET（点名锁定形态），但 FIDELITY=behavior 只承诺行为一致",
			Fix:      "若这些片段确实要求逐字复现，把 FIDELITY 升为 structure 或 artifact；否则删除未点名的 SNIPPET",
		})
	}

	// 2. CONTRACT negation vs ACCEPT usage of the same feature.
	negSet := map[string]bool{}
	contract := d.Section("CONTRACT")
	if contract != nil {
		for _, line := range contract.Lines {
			for _, t := range negatedTokens(line) {
				for k := range operatorTokens(t) {
					negSet[k] = true
				}
			}
		}
	}
	if accept := d.Section("ACCEPT"); accept != nil && len(negSet) > 0 {
		for _, e := range accept.Entries() {
			used := operatorTokens(e.Text)
			for k := range used {
				if negSet[k] {
					out = append(out, Diagnostic{
						Severity: SevError,
						Section:  "ACCEPT",
						ID:       "A" + e.ID,
						Msg:      fmt.Sprintf("验收用例用到被 CONTRACT 否定的功能(%s)", k),
						Fix:      "要么从 ACCEPT 移除该用例，要么 CONTRACT 的否定是错的——两者只能留一个",
					})
				}
			}
		}
	}

	// 3. DECISIONS reject item still required in CONTRACT.
	rejected := map[string]bool{}
	if dec := d.Section("DECISIONS"); dec != nil {
		for _, line := range dec.Lines {
			if i := strings.Index(strings.ToLower(line), "reject:"); i >= 0 {
				for k := range operatorTokens(line[i:]) {
					rejected[k] = true
				}
			}
		}
	}
	if contract != nil && len(rejected) > 0 {
		for _, e := range contract.Entries() {
			used := operatorTokens(e.Text)
			for k := range used {
				if rejected[k] {
					out = append(out, Diagnostic{
						Severity: SevError,
						Section:  "CONTRACT",
						ID:       "R" + e.ID,
						Msg:      fmt.Sprintf("DECISIONS 已否决 %s，但契约仍要求它", k),
						Fix:      "删除契约中的该功能，或先推翻对应 DECISIONS（一次新对话）",
					})
				}
			}
		}
	}

	// 4. META.spec must match current spec version when present.
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
