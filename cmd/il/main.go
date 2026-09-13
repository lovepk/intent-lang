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
		return fmt.Errorf("usage: il <chat|verify|repro|accept|lint|demo|log|show|rollback> [options]")
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
		return fmt.Errorf("compare 已废弃：文本相似度不能衡量意图一致性。请用 accept（ACCEPT 行为验收）或 lint")
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
	name := f["name"]
	if name == "" {
		name = archive.DefaultName
	}
	return archive.OpenArchive(dir, name)
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

	fmt.Printf("il chat [%s] via %s（repo: %s）— 输入需求；空行或 exit 退出\n", store.Name(), p.Name(), store.Dir())
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
		// L0 生成：LLM 自由产出草稿，不做任何约束。
		resp, err := p.Complete(ctx, provider.Request{System: il.SpecPrompt, Archive: beforeDoc, User: msg})
		if err != nil {
			fmt.Println("!!", err)
			stats.noteError(err)
			continue
		}
		if strings.TrimSpace(resp.IntentUpdate) == "" {
			fmt.Println("--- reply ---")
			fmt.Println(resp.Reply)
			fmt.Println("(档案未变更)")
			continue
		}
		// L1 确定性检查 + L2 记录员（仅 L1 报警时触发）：只记录，不拒绝、不提示。
		after, normalized := recordArchive(ctx, p, resp.IntentUpdate, beforeDoc)
		if normalized {
			stats.Normalizations++
		}
		summary := archive.Summarize(beforeRaw, after)
		commits := lenLog(store) + 1
		deprecated = nextDeprecated(deprecated, beforeRaw, after)
		c, err := store.Append(summary, msg, beforeRaw, after, resp.Reply,
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
		fmt.Println("--- reply ---")
		fmt.Println(resp.Reply)
		printAuthorityPanel(summary)
		fmt.Printf("--- intent_update（commit %s）---\n", c.ID)
		fmt.Println(noMeta(c.Archive))
	}

	if cur != "" {
		out := filepath.Join(store.ArchiveDir(), "archive.last.il")
		if err := os.WriteFile(out, []byte(cur), 0o644); err != nil {
			return err
		}
		fmt.Printf("最终档案已写入 %s（累计 %d 次提交）\n", out, lenLog(store))
	}
	fmt.Print(stats.render())
	return nil
}
