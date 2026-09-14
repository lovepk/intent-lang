package main

import (
	"fmt"
	"strings"

	"github.com/anomalyco/intent-lang/internal/il"
)

// ComplianceStats records generation errors and consistency-lint findings. This
// feeds back into spec decisions (which constraints LLMs violate most).
type ComplianceStats struct {
	GenerateErrors      map[string]int
	LintFindings        map[string]int
	LintErrorCount      int
	LintSuggestionCount int
	Commits             int
	Normalizations      int
}

func newStats() *ComplianceStats {
	return &ComplianceStats{
		GenerateErrors: map[string]int{},
		LintFindings:   map[string]int{},
	}
}

func (s *ComplianceStats) noteError(err error) {
	s.GenerateErrors[classify(err.Error())]++
}

func (s *ComplianceStats) noteLint(doc *il.Doc) {
	for _, d := range doc.Lint() {
		key := d.Section
		if d.ID != "" {
			key += " " + d.ID
		}
		s.LintFindings[key]++
		switch d.Severity {
		case il.SevError:
			s.LintErrorCount++
		default:
			s.LintSuggestionCount++
		}
	}
}

func classify(msg string) string {
	if strings.Contains(msg, "JSON") {
		return "invalid json"
	}
	return "other"
}

func (s *ComplianceStats) render() string {
	var b strings.Builder
	b.WriteString("\n=== 规范遵守统计 ===\n")
	if s.Commits > 0 {
		fmt.Fprintf(&b, "提交数: %d\n", s.Commits)
	}
	if s.Normalizations > 0 {
		fmt.Fprintf(&b, "L2 记录员触发: %d 次（L1 发现非法/矛盾）\n", s.Normalizations)
	}
	b.WriteString("生成错误(按类型):\n")
	if len(s.GenerateErrors) == 0 {
		b.WriteString("  无\n")
	}
	for k, v := range s.GenerateErrors {
		fmt.Fprintf(&b, "  %s: %d\n", k, v)
	}
	b.WriteString("一致性 Lint 发现(按位置):\n")
	if s.LintErrorCount == 0 && s.LintSuggestionCount == 0 {
		b.WriteString("  无\n")
	} else {
		fmt.Fprintf(&b, "  合计 %d error / %d suggestion\n", s.LintErrorCount, s.LintSuggestionCount)
	}
	for k, v := range s.LintFindings {
		fmt.Fprintf(&b, "  %s: %d\n", k, v)
	}
	return b.String()
}
