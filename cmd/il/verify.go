package main

import (
	"context"
	"fmt"
	"strings"

	"intent-lang/internal/il"
	"intent-lang/internal/provider"
)

// verify previews, without committing, how the model would rewrite an archive
// in response to a request. It is chat's stateless sibling: it reads an
// archive (file or repo/name), applies the change gates, and reports the
// declared vs actual changes — but writes nothing.
func verify(args []string) error {
	f := flags(args)
	p, err := makeProvider(f)
	if err != nil {
		return err
	}

	msg := f["msg"]
	if msg == "" {
		msg = positionalArg(args)
	}
	if msg == "" {
		return fmt.Errorf("verify requires --msg <用户需求>")
	}

	// load source (unresolved, so the model sees refs and may update them).
	// An empty archive (no commits yet / no archive given) means "start from
	// scratch" and is valid for a preview.
	source := ""
	if f["archive"] != "" || f["repo"] != "" || f["name"] != "" {
		s, err := loadSourceRaw(f)
		if err != nil {
			if !strings.Contains(err.Error(), "empty") {
				return fmt.Errorf("load archive: %w", err)
			}
			source = ""
		} else {
			source = s
		}
	}
	beforeDoc := noMeta(source)

	ctx := context.Background()
	resp, err := p.Complete(ctx, provider.Request{System: il.SpecPrompt, Archive: beforeDoc, User: msg})
	if err != nil {
		return err
	}

	fmt.Println("== reply ==")
	fmt.Println(resp.Reply)

	if strings.TrimSpace(resp.IntentUpdate) == "" {
		fmt.Println("== 判定 ==\n(档案未变更)")
		return nil
	}
	after := resp.IntentUpdate

	// structural validation of the proposed rewrite
	if errs := validateIL(after); errs != nil {
		fmt.Println("== 判定 ==")
		fmt.Printf("!! 模型输出不是合法档案（%d 处）:\n", len(errs))
		for _, e := range errs {
			fmt.Println("  [error] " + e.Error())
		}
		return nil
	}

	// change gate: actual machine diff vs declared changes
	if over := undeclaredChanges(source, after, resp.DeclaredChanges); len(over) > 0 {
		fmt.Println("== 判定：变更闸门会拦截 ==")
		for _, c := range over {
			fmt.Printf("  [%s] %s\n", c.Kind, c.ID)
		}
		fmt.Println("模型改了这些条目但未在 declared_changes 声明。若经 chat 提交会被拒绝。")
	} else {
		fmt.Println("== 判定：可通过变更闸门 ==")
		if len(resp.DeclaredChanges) > 0 {
			fmt.Println("声明的改动: " + strings.Join(resp.DeclaredChanges, ", "))
		} else {
			fmt.Println("(无档案变更声明)")
		}
	}

	fmt.Println("== 模型重写后的档案（预览，未提交） ==")
	fmt.Println(noMeta(after))
	return nil
}
