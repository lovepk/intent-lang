package main

import (
	"context"
	"fmt"

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
	source, err := loadSourceRaw(f)
	if err != nil {
		return err
	}
	doc, err := il.Parse(source)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	errs := doc.Validate()
	if len(errs) > 0 {
		fmt.Printf("语法校验: %d 个硬伤\n", len(errs))
		for _, e := range errs {
			fmt.Println("  [error] " + e.Error())
		}
		fmt.Println("建议先用 Retry 机制让模型修正，或人工修正后再 lint。")
		return nil
	}
	fmt.Println("语法校验: 通过")
	fmt.Print(doc.LintString())

	if f["llm"] == "true" {
		fmt.Println("\n=== LLM 语义复查 ===")
		p, err := makeProvider(f)
		if err != nil {
			return err
		}
		p = retryProvider(p, f)
		ctx := context.Background()
		resp, err := p.Complete(ctx, provider.Request{
			System:  il.LintPrompt,
			Archive: doc.Canonical(),
			User:    "请复查这份档案的语义一致性。",
			Mode:    provider.ModeLint,
		})
		if err != nil {
			return err
		}
		if len(resp.Findings) == 0 {
			fmt.Println("LLM 复查: 未发现语义矛盾")
			return nil
		}
		for _, fi := range resp.Findings {
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
	return nil
}
