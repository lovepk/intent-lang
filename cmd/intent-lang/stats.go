package main

import (
	"fmt"
	"strings"

	"intent-lang/internal/il"
)

// ComplianceStats records how often the LLM produced archives that tripped
// validation or lint, categorized by message. This feeds back into spec v2
// decisions (which constraints LLMs violate most).
type ComplianceStats struct {
	ValidateRejections map[string]int
	LintFindings       map[string]int
	Commits            int
}

func newStats() *ComplianceStats {
	return &ComplianceStats{
		ValidateRejections: map[string]int{},
		LintFindings:       map[string]int{},
	}
}

func (s *ComplianceStats) noteValidate(errs []error) {
	for _, e := range errs {
		s.ValidateRejections[classify(e.Error())]++
	}
}

func (s *ComplianceStats) noteLint(doc *il.Doc) {
	for _, d := range doc.Lint() {
		key := d.Section
		if d.ID != "" {
			key += " " + d.ID
		}
		s.LintFindings[key]++
	}
}

func classify(msg string) string {
	switch {
	case strings.Contains(msg, "missing header"):
		return "missing header"
	case strings.Contains(msg, "invalid FIDELITY"):
		return "invalid fidelity"
	case strings.Contains(msg, "not increasing"):
		return "id not increasing"
	case strings.Contains(msg, "duplicate body"):
		return "duplicate body"
	case strings.Contains(msg, "missing default"):
		return "open missing default"
	case strings.Contains(msg, "empty section"):
		return "empty section"
	case strings.Contains(msg, "duplicate section"):
		return "duplicate section"
	case strings.Contains(msg, "JSON"):
		return "invalid json"
	default:
		return "other"
	}
}

func (s *ComplianceStats) render() string {
	var b strings.Builder
	b.WriteString("\n=== 规范遵守统计 ===\n")
	if s.Commits > 0 {
		fmt.Fprintf(&b, "提交数: %d\n", s.Commits)
	}
	b.WriteString("语法校验拒绝(按类型):\n")
	if len(s.ValidateRejections) == 0 {
		b.WriteString("  无\n")
	}
	for k, v := range s.ValidateRejections {
		fmt.Fprintf(&b, "  %s: %d\n", k, v)
	}
	b.WriteString("一致性 Lint 发现(按位置):\n")
	if len(s.LintFindings) == 0 {
		b.WriteString("  无\n")
	}
	for k, v := range s.LintFindings {
		fmt.Fprintf(&b, "  %s: %d\n", k, v)
	}
	return b.String()
}
