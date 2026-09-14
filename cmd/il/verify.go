package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/lovepk/intent-lang/internal/agent"
	"github.com/lovepk/intent-lang/internal/archive"
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
		s, err := agent.LoadSourceRaw(sourceOpts(f))
		if err != nil {
			if !strings.Contains(err.Error(), "empty") {
				return fmt.Errorf("load archive: %w", err)
			}
			source = ""
		} else {
			source = s
		}
	}

	turn, err := (&agent.Agent{Model: p}).Record(context.Background(), source, msg)
	if err != nil {
		return err
	}

	fmt.Println("== reply ==")
	fmt.Println(turn.Reply)

	if !turn.Changed {
		fmt.Println("== 判定 ==\n(档案未变更)")
		return nil
	}
	if turn.Normalized {
		fmt.Println("== L1 发现问题，L2 记录员已整理为合法自洽档案 ==")
	}

	printAuthorityPanel(archive.Summarize(source, turn.Archive))

	fmt.Println("== 档案（预览，未提交） ==")
	fmt.Println(agent.NoMeta(turn.Archive))
	return nil
}
