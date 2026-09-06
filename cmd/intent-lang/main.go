package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	case "compare":
		return compare(os.Args[2:])
	case "accept":
		return accept(os.Args[2:])
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
		dir = ".intent"
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

func verifyMeta(before, after string) error { return nil }

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

func buildMeta(commits int) []string {
	return []string{
		"spec: v1.0",
		"created: " + time.Now().UTC().Format(time.RFC3339),
		fmt.Sprintf("commits: %d", commits),
		"deprecated: []",
	}
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

		beforeRaw := cur
		beforeDoc := noMeta(beforeRaw)
		resp, err := p.Complete(ctx, provider.Request{System: il.SpecPrompt, Archive: beforeDoc, User: msg})
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
		after := resp.IntentUpdate
		if errs := validateIL(after); errs != nil {
			fmt.Println("!! 输出不是合法档案，已拒绝：", errs)
			continue
		}
		commits++
		c, err := store.Append(archive.Summarize(beforeRaw, after), msg, beforeRaw, after, resp.Reply, buildMeta(commits))
		if err != nil {
			return err
		}
		cur = c.Archive
		fmt.Printf("--- intent_update（commit %s）---\n", c.ID)
		fmt.Println(noMeta(c.Archive))
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
