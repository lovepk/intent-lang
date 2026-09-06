package il

import (
	"fmt"
	"regexp"
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

// refRe matches cross-references to entries: 见R3 / 参见A1 / 参考?2 / 如 D1.
var refRe = regexp.MustCompile(`(?:见|参见|参考|如|引用)\s*([RAD]\d+|\?\d+)`)

// entrySets indexes ids per section kind for dangle checks.
func entrySets(d *Doc) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, name := range []string{"CONTRACT", "ACCEPT", "DECISIONS", "OPEN"} {
		out[name] = map[string]bool{}
		if s := d.Section(name); s != nil {
			for _, e := range s.Entries() {
				prefix := "R"
				switch name {
				case "ACCEPT":
					prefix = "A"
				case "DECISIONS":
					prefix = "D"
				case "OPEN":
					prefix = "?"
				}
				out[name][prefix+e.ID] = true
			}
		}
	}
	return out
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

	// Dangling cross-references: 见 R3 / 参见 A2 / 参考 ?1 pointing nowhere.
	sets := entrySets(d)
	for _, name := range []string{"CONTRACT", "ACCEPT", "DECISIONS", "OPEN"} {
		s := d.Section(name)
		if s == nil {
			continue
		}
		for _, e := range s.Entries() {
			for _, m := range refRe.FindAllStringSubmatch(e.Text, -1) {
				ref := m[1]
				targetName := sectionForRef(ref)
				if !sets[targetName][ref] {
					out = append(out, Diagnostic{
						Severity: SevError,
						Section:  name,
						ID:       entryPrefixOf(name) + e.ID,
						Msg:      fmt.Sprintf("引用了不存在的条目 %s", ref),
						Fix:      "修正引用目标，或确认该条目未被删除",
					})
				}
			}
		}
	}

	// OPEN items whose default duplicates an already-decided CONTRACT fact:
	// a likely convergence the dialogue forgot to close. Heuristic: non-empty
	// default whose wording is fully contained in some CONTRACT body.
	if open := d.Section("OPEN"); open != nil {
		contract := d.Section("CONTRACT")
		if contract != nil {
			contractBodies := map[string]bool{}
			for _, e := range contract.Entries() {
				contractBodies[entryBody(e.Text)] = true
			}
			for _, line := range open.Lines {
				body := entryBody(line)
				if i := strings.Index(body, "default:"); i >= 0 {
					def := strings.TrimSpace(strings.TrimPrefix(body[i:], "default:"))
					def = trimQuote(def)
					if len(def) < 2 {
						continue
					}
					for cb := range contractBodies {
						if strings.Contains(cb, def) {
							out = append(out, Diagnostic{
								Severity: SevSuggestion,
								Section:  "OPEN",
								Msg:      fmt.Sprintf("开放项的 default（%s）已被 CONTRACT 决定（%s）", def, shortLine(cb)),
								Fix:      "该点已消歧，应把 OPEN 项移除或移入 DECISIONS，而不是保留",
							})
							break
						}
					}
				}
			}
		}
	}

	return out
}

func trimQuote(s string) string {
	return strings.Trim(s, "\"'“”‘’")
}

func shortLine(s string) string {
	if len([]rune(s)) > 40 {
		r := []rune(s)
		return string(r[:40]) + "…"
	}
	return s
}

func sectionForRef(ref string) string {
	switch ref[0] {
	case 'R':
		return "CONTRACT"
	case 'A':
		return "ACCEPT"
	case 'D':
		return "DECISIONS"
	default:
		return "OPEN"
	}
}

func entryPrefixOf(name string) string {
	switch name {
	case "CONTRACT":
		return "R"
	case "ACCEPT":
		return "A"
	case "DECISIONS":
		return "D"
	default:
		return "?"
	}
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
