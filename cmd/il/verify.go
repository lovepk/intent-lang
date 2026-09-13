package main

import (
	"context"
	"fmt"
	"strings"

	"intent-lang/internal/archive"
	"intent-lang/internal/il"
	"intent-lang/internal/provider"
)

// verify previews, without committing, how the model would rewrite an archive
// in response to a request. It is chat's stateless sibling: it runs the same
// layered checks (L1 deterministic, L2 recorder when needed) and shows the
// machine diff — but writes nothing.
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

	// L1 确定性检查 + L2 记录员（仅 L1 报警时触发）：与 chat 同一套分层逻辑。
	after, normalized := recordArchive(ctx, p, resp.IntentUpdate, beforeDoc)
	if normalized {
		fmt.Println("== L1 发现问题，L2 记录员已整理为合法自洽档案 ==")
	}

	printAuthorityPanel(archive.Summarize(source, after))

	fmt.Println("== 档案（预览，未提交） ==")
	fmt.Println(noMeta(after))
	return nil
}
