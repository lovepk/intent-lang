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
		return fmt.Errorf("usage: intent-lang <chat|verify|repro|log|show|rollback> [options]")
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
		dir = ".intent"
	}
	return archive.Open(dir)
}

func makeProvider(f map[string]string) (provider.Provider, error) {
	kind := f["provider"]
	if kind == "" {
		kind = "deepseek"
	}
	switch kind {
	case "mock":
		return provider.NewMock("mock-calculus"), nil
	case "deepseek":
		keyEnv := "DEEPSEEK_API_KEY_A"
		if f["key"] == "B" {
			keyEnv = "DEEPSEEK_API_KEY_B"
		}
		return provider.NewDeepSeekFromEnv(keyEnv), nil
	default:
		return nil, fmt.Errorf("unknown provider %q", kind)
	}
}

func retryProvider(p provider.Provider, f map[string]string) provider.Provider {
	attempts := 2
	if f["retry"] != "" {
		fmt.Sscanf(f["retry"], "%d", &attempts)
	}
	return provider.NewRetry(p, attempts)
}

func verifyMeta(before, after string) error {
	if strings.TrimSpace(before) == "" {
		return nil
	}
	b, err := il.Parse(before)
	if err != nil {
		return err
	}
	a, err := il.Parse(after)
	if err != nil {
		return err
	}
	if !il.MetaUnchanged(b, a) {
		return fmt.Errorf("LLM 修改了 META 段，非法")
	}
	return nil
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
	if latest != nil {
		cur = latest.Archive
	}
	commits := lenLog(store)

	fmt.Printf("intent-lang chat via %s（repo: %s）— 输入需求；空行或 exit 退出\n", p.Name(), store.Dir())
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		msg := strings.TrimSpace(scanner.Text())
		if msg == "" || msg == "exit" || msg == "quit" {
			break
		}

		resp, err := p.Complete(ctx, provider.Request{System: il.SpecPrompt, Archive: cur, User: msg})
		if err != nil {
			fmt.Println("!!", err)
			continue
		}
		fmt.Println("--- reply ---")
		fmt.Println(resp.Reply)
		if strings.TrimSpace(resp.IntentUpdate) == "" {
			fmt.Println("(档案未变更)")
			continue
		}
		before := cur
		after := resp.IntentUpdate
		if err := verifyMeta(before, after); err != nil {
			fmt.Println("!! META 被改动，本次变更已拒绝：", err)
			continue
		}
		commits++
		c, err := store.Append(archive.Summarize(before, after), msg, before, after, resp.Reply)
		if err != nil {
			return err
		}
		cur = after
		fmt.Printf("--- intent_update（commit %s）---\n", c.ID)
		fmt.Println(after)
	}

	if cur != "" {
		out := filepath.Join(store.Dir(), "archive.last.intent")
		if err := os.WriteFile(out, []byte(cur), 0o644); err != nil {
			return err
		}
		fmt.Printf("最终档案已写入 %s（累计 %d 次提交）\n", out, commits)
	}
	return nil
}
