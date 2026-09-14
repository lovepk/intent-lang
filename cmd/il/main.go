package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anomalyco/intent-lang/internal/agent"
	"github.com/anomalyco/intent-lang/internal/archive"
	"github.com/anomalyco/intent-lang/internal/il"
	"github.com/anomalyco/intent-lang/internal/provider"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: il <chat|verify|repro|accept|lint|fmt|demo|mcp|log|show|rollback> [options]")
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
	case "accept":
		return accept(os.Args[2:])
	case "lint":
		return lint(os.Args[2:])
	case "fmt":
		return fmtCmd(os.Args[2:])
	case "demo":
		return demo(os.Args[2:])
	case "mcp":
		return cmdMCP(os.Args[2:])
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
	return agent.OpenStore(sourceOpts(f))
}

func sourceOpts(f map[string]string) agent.SourceOpts {
	return agent.SourceOpts{Archive: f["archive"], Repo: f["repo"], Name: f["name"]}
}

func makeProvider(f map[string]string) (provider.Provider, error) {
	return provider.FromEnv(f["provider"], f["key"])
}

func providerName(f map[string]string) string {
	if n := strings.TrimSpace(f["provider"]); n != "" {
		return n
	}
	return "deepseek"
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

	store, err := openStore(f)
	if err != nil {
		return err
	}

	ctx := context.Background()
	ag := &agent.Agent{Model: p}
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
		createdCarry, deprecated = agent.ParseMetaCarry(cur)
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
		// L0 生成 + L1/L2：模型自由产出草稿；只记录，不拒绝、不提示。
		turn, err := ag.Record(ctx, beforeRaw, msg)
		if err != nil {
			fmt.Println("!!", err)
			stats.noteError(err)
			continue
		}
		if !turn.Changed {
			fmt.Println("--- reply ---")
			fmt.Println(turn.Reply)
			fmt.Println("(档案未变更)")
			continue
		}
		if turn.Normalized {
			stats.Normalizations++
		}
		after := turn.Archive
		summary := archive.Summarize(beforeRaw, after)
		commits := lenLog(store) + 1
		deprecated = agent.NextDeprecated(deprecated, beforeRaw, after)
		c, err := store.Append(summary, msg, beforeRaw, after, turn.Reply,
			agent.BuildMetaLines(commits, createdCarry, deprecated))
		if err != nil {
			return err
		}
		cur = c.Archive
		createdCarry, _ = agent.ParseMetaCarry(cur)
		stats.Commits++
		if doc, perr := il.Parse(agent.NoMeta(cur)); perr == nil {
			stats.noteLint(doc)
		}
		fmt.Println("--- reply ---")
		fmt.Println(turn.Reply)
		printAuthorityPanel(summary)
		fmt.Printf("--- intent_update（commit %s）---\n", c.ID)
		fmt.Println(agent.NoMeta(c.Archive))
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
