package main

import (
	"context"
	"fmt"
	"strings"

	"intent-lang/internal/agent"
	"intent-lang/internal/il"
	"intent-lang/internal/provider"
)

func lint(args []string) error {
	f := flags(args)
	if f["archive"] == "" {
		f["archive"] = positionalArg(args)
	}
	if f["archive"] == "" && f["repo"] == "" && f["name"] == "" {
		return fmt.Errorf("lint requires --archive <file.il> 或 --repo <dir> [--name <档案名>]")
	}
	source, err := agent.LoadSourceRaw(sourceOpts(f))
	if err != nil {
		return err
	}
	doc, err := il.Parse(source)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	failOn := strings.TrimSpace(f["fail-on"])

	errs := doc.Validate()
	if len(errs) > 0 {
		fmt.Printf("语法校验: %d 个硬伤\n", len(errs))
		for _, e := range errs {
			fmt.Println("  [error] " + e.Error())
		}
		if failOn == "error" {
			return fmt.Errorf("lint: %d 个结构硬伤", len(errs))
		}
		fmt.Println("请人工修正后再 lint。")
		return nil
	}
	fmt.Println("语法校验: 通过")

	diags := doc.Lint()
	fmt.Print(renderDiags(diags))

	if f["llm"] == "true" {
		p, err := makeProvider(f)
		if err != nil {
			return err
		}
		fmt.Println("\n=== LLM 语义复查（档案内部） ===")
		findings, err := (&agent.Agent{Model: p}).LintLLM(context.Background(), doc.Canonical())
		if err != nil {
			return err
		}
		printFindings(findings)
	}

	if failOn == "error" {
		if n := countErrors(diags); n > 0 {
			return fmt.Errorf("lint: %d 处 error", n)
		}
	}
	return nil
}

func countErrors(diags []il.Diagnostic) int {
	n := 0
	for _, d := range diags {
		if d.Severity == il.SevError {
			n++
		}
	}
	return n
}

func renderDiags(diags []il.Diagnostic) string {
	if len(diags) == 0 {
		return "lint: 通过，未发现一致性矛盾\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "lint: 发现 %d 处（%d error / %d suggestion）\n",
		len(diags), countErrors(diags), len(diags)-countErrors(diags))
	for _, d := range diags {
		b.WriteString("  " + d.String() + "\n")
	}
	return b.String()
}

// printFindings renders LLM lint findings.
func printFindings(findings []provider.Finding) {
	if len(findings) == 0 {
		fmt.Println("LLM 复查: 未发现问题")
		return
	}
	for _, fi := range findings {
		sev := fi.Severity
		if sev == "" {
			sev = "suggestion"
		}
		where := fi.Section
		if fi.ID != "" {
			where += " " + fi.ID
		}
		fmt.Printf("[%s] %s: %s\n", sev, where, fi.Msg)
		if fi.Fix != "" {
			fmt.Println("  建议: " + fi.Fix)
		}
	}
}
