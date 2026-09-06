package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

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
		return fmt.Errorf("usage: intent-lang <chat|verify|repro> [--provider mock|deepseek] [--key A|B]")
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

func chat(args []string) error {
	f := flags(args)
	p, err := makeProvider(f)
	if err != nil {
		return err
	}

	ctx := context.Background()
	scanner := bufio.NewScanner(os.Stdin)
	archive := ""
	version := 0

	fmt.Printf("intent-lang chat via %s — 输入需求；空行或 exit 退出\n", p.Name())
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		msg := strings.TrimSpace(scanner.Text())
		if msg == "" || msg == "exit" || msg == "quit" {
			break
		}

		resp, err := p.Complete(ctx, provider.Request{System: il.SpecPrompt, Archive: archive, User: msg})
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
		version++
		archive = resp.IntentUpdate
		fmt.Printf("--- intent_update v%d ---\n", version)
		fmt.Println(archive)
	}

	if archive != "" {
		out := "archive.last.intent"
		if err := os.WriteFile(out, []byte(archive), 0o644); err != nil {
			return err
		}
		fmt.Printf("最终档案已写入 %s（%d 次提交）\n", out, version)
	}
	return nil
}
