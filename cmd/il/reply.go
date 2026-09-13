package main

import (
	"fmt"
	"strings"

	"intent-lang/internal/il"
)

// reportReplyClaims prints deterministic reply↔archive consistency findings.
// Advisory only: it never blocks a commit.
func reportReplyClaims(reply, archiveText string, declared []string) {
	findings := il.CheckReplyClaims(reply, archiveText, declared)
	if len(findings) == 0 {
		return
	}
	fmt.Println("!! reply↔档案一致性提示（advisory）：")
	for _, f := range findings {
		fmt.Printf("   [%s] %s\n", f.Severity, f.Msg)
	}
}

// printAuthorityPanel shows the machine-computed change summary as the
// authoritative record of what changed, independent of the free-text reply.
func printAuthorityPanel(summary string) {
	if strings.TrimSpace(summary) == "" {
		return
	}
	fmt.Printf("--- 档案实际变更（权威，机器 diff）---\n%s\n", summary)
}
