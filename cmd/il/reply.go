package main

import (
	"fmt"
	"strings"
)

// printAuthorityPanel shows the machine-computed change summary (L1) as the
// authoritative record of what changed this turn.
func printAuthorityPanel(summary string) {
	if strings.TrimSpace(summary) == "" {
		return
	}
	fmt.Printf("--- 档案实际变更（权威，机器 diff）---\n%s\n", summary)
}
