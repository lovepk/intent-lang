package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"intent-lang/internal/archive"
	"intent-lang/internal/il"
	"intent-lang/internal/provider"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: il <chat|verify|repro|compare|accept|lint|demo|log|show|rollback> [options]")
	}
	cmd := os.Args[1]

	loadDotEnv()

	switch cmd {
	case "chat":
		return chat(os.Args[2:])
	case "verify":
		return verify(os.Args[2:])
	case "repro":
		return repro(os.Args[2:])
	case "compare":
		return compare(os.Args[2:])
	case "accept":
		return accept(os.Args[2:])
	case "lint":
		return lint(os.Args[2:])
	case "demo":
		return demo(os.Args[2:])
	case "log":
		return cmdLog(os.Args[2:])
	case "show":
		return cmdShow(os.Args[2:])
	case "rollback":
		return cmdRollback(os.Args[2:])
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func flags(args []string) map[string]string {
	out := map[string]string{}
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--") {
			key := strings.TrimPrefix(args[i], "--")
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				out[key] = args[i+1]
				i++
			} else {
				out[key] = "true"
			}
		}
	}
	return out
}

func openStore(f map[string]string) (*archive.Store, error) {
	dir := f["repo"]
	if dir == "" {
		dir = ".il"
	}
	return archive.Open(dir)
}

func makeProvider(f map[string]string) (provider.Provider, error) {
	return provider.NewDeepSeekFromEnv(envKeyFor(f["key"])), nil
}

func envKeyFor(key string) string {
	if key == "B" {
		return "DEEPSEEK_API_KEY_B"
	}
	return "DEEPSEEK_API_KEY_A"
}

func retryProvider(p provider.Provider, f map[string]string) provider.Provider {
	attempts := 2
	if f["retry"] != "" {
		fmt.Sscanf(f["retry"], "%d", &attempts)
	}
	return provider.NewRetry(p, attempts)
}

func noMeta(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	doc, err := il.Parse(text)
	if err != nil {
		return text
	}
	return doc.StripMeta().Canonical()
}

func validateIL(text string) []error {
	doc, err := il.Parse(text)
	if err != nil {
		return []error{err}
	}
	return doc.Validate()
}

func lenLog(store *archive.Store) int {
	log, err := store.Log()
	if err != nil {
		return 0
	}
	return len(log)
}

func chat(args []string) error {
	f := flags(args)
	p, err := makeProvider(f)
	if err != nil {
		return err
	}
	p = retryProvider(p, f)

	store, err := openStore(f)
	if err != nil {
		return err
	}

	ctx := context.Background()
	scanner := bufio.NewScanner(os.Stdin)
	latest, err := store.Latest()
	if err != nil {
		return err
	}
	cur := ""
	createdCarry := ""
	var deprecated []string
	if latest != nil {
		cur = latest.Archive
		createdCarry, deprecated = parseMetaCarry(cur)
	}
	stats := newStats()

	fmt.Printf("il chat via %s（repo: %s）— 输入需求；空行或 exit 退出\n", p.Name(), store.Dir())
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		msg := strings.TrimSpace(scanner.Text())
		if msg == "" || msg == "exit" || msg == "quit" {
			break
		}

		beforeRaw := cur
		beforeDoc := noMeta(beforeRaw)
		if resp, err := p.Complete(ctx, provider.Request{System: il.SpecPrompt, Archive: beforeDoc, User: msg}); err != nil {
			fmt.Println("!!", err)
			stats.noteValidate([]error{err})
			continue
		} else {
			fmt.Println("--- reply ---")
			fmt.Println(resp.Reply)
			if strings.TrimSpace(resp.IntentUpdate) == "" {
				fmt.Println("(档案未变更)")
				continue
			}
			after := resp.IntentUpdate
			if errs := validateIL(after); errs != nil {
				fmt.Println("!! 输出不是合法档案，已拒绝：", errs)
				stats.noteValidate(errs)
				continue
			}
			// 变更闸门：模型声明了改哪些条目；机器 diff 若发现"改了却没声明"，
			// 视为越权改动，拒绝落库（确定性守卫，保护已消歧事实不被顺手改动）。
			if over := undeclaredChanges(beforeRaw, after, resp.DeclaredChanges); len(over) > 0 {
				fmt.Println("!! 变更闸门拒绝：模型改动以下条目但未在 declared_changes 中声明：", over)
				fmt.Println("   已消歧事实可能被顺手改动。请重新表达需求让模型如实声明改动，或人工核对。")
				stats.noteValidate([]error{fmt.Errorf("undeclared changes: %v", over)})
				continue
			}
			commits := lenLog(store) + 1
			deprecated = nextDeprecated(deprecated, beforeRaw, after)
			c, err := store.Append(archive.Summarize(beforeRaw, after), msg, beforeRaw, after, resp.Reply,
				buildMetaLines(commits, createdCarry, deprecated))
			if err != nil {
				return err
			}
			cur = c.Archive
			createdCarry, _ = parseMetaCarry(cur)
			stats.Commits++
			if doc, perr := il.Parse(noMeta(cur)); perr == nil {
				stats.noteLint(doc)
			}
			fmt.Printf("--- intent_update（commit %s）---\n", c.ID)
			fmt.Println(noMeta(c.Archive))
		}
	}

	if cur != "" {
		out := filepath.Join(store.Dir(), "archive.last.il")
		if err := os.WriteFile(out, []byte(cur), 0o644); err != nil {
			return err
		}
		fmt.Printf("最终档案已写入 %s（累计 %d 次提交）\n", out, lenLog(store))
	}
	fmt.Print(stats.render())
	return nil
}
