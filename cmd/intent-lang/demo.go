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
	"intent-lang/internal/similarity"
)

// demo runs the full golden path non-interactively:
// key A builds archive from a script file → key B reproduces → ACCEPT run.
func demo(args []string) error {
	f := flags(args)
	turnsFile := f["script"]
	if turnsFile == "" {
		return fmt.Errorf("demo requires --script <turns.txt>")
	}
	repoDir := f["repo"]
	if repoDir == "" {
		repoDir = filepath.Join(os.TempDir(), "il-demo-repo")
	}
	os.RemoveAll(repoDir)

	keyA := provider.NewRetry(provider.NewDeepSeekFromEnv(envKeyFor("A")), 2)
	keyB := provider.NewRetry(provider.NewDeepSeekFromEnv(envKeyFor("B")), 2)

	store, err := archive.Open(repoDir)
	if err != nil {
		return err
	}
	ctx := context.Background()

	fmt.Println("=== 阶段 1: key A 双通道建档 ===")
	cur := ""
	latest, _ := store.Latest()
	if latest != nil {
		cur = latest.Archive
	}
	file, err := os.Open(turnsFile)
	if err != nil {
		return err
	}
	defer file.Close()
	sc := bufio.NewScanner(file)
	sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	createdCarry := ""
	var deprecated []string
	for sc.Scan() {
		msg := strings.TrimSpace(sc.Text())
		if msg == "" {
			continue
		}
		resp, err := keyA.Complete(ctx, provider.Request{
			System:  il.SpecPrompt,
			Archive: noMeta(cur),
			User:    msg,
		})
		if err != nil {
			fmt.Println("!!", err)
			continue
		}
		if strings.TrimSpace(resp.IntentUpdate) == "" {
			fmt.Printf("turn: %s\n  reply: %s\n  (档案未变更)\n", msg, resp.Reply)
			continue
		}
		before := cur
		after := resp.IntentUpdate
		deprecated = nextDeprecated(deprecated, before, after)
		c, err := store.Append(archive.Summarize(before, after), msg, before, after, resp.Reply,
			buildMetaLines(lenLog(store)+1, createdCarry, deprecated))
		if err != nil {
			return err
		}
		cur = c.Archive
		createdCarry, _ = parseMetaCarry(cur)
		fmt.Printf("turn: %s\n  reply: %s\n  commit %s: %s\n", msg, resp.Reply, c.ID, c.Message)
	}
	if cur == "" {
		return fmt.Errorf("demo: no archive produced")
	}
	archiveOut := filepath.Join(repoDir, "archive.last.intent")
	if err := os.WriteFile(archiveOut, []byte(cur), 0o644); err != nil {
		return err
	}

	fmt.Println("\n=== 阶段 2: 删会话，key B 仅凭档案复现 ===")
	doc, err := il.Parse(cur)
	if err != nil {
		return err
	}
	reproResp, err := keyB.Complete(ctx, provider.Request{
		System:  il.ReproPrompt,
		Archive: doc.Canonical(),
		User:    "请据此档案重建产物。",
		Mode:    provider.ModeRepro,
	})
	if err != nil {
		return err
	}
	artifact := provider.StripFence(reproResp.Reply)
	artifactFile := filepath.Join(repoDir, "artifact"+artifactExt(doc.HeaderRaw["TARGET"]))
	if err := os.WriteFile(artifactFile, []byte(artifact), 0o644); err != nil {
		return err
	}
	fmt.Printf("复现产物已写入 %s（%d 字节）\n", artifactFile, len(artifact))

	fmt.Println("\n=== 阶段 3: ACCEPT 行为验收 ===")
	acceptF := map[string]string{"archive": archiveOut, "artifact": artifactFile}
	if err := accept(flagSlice(acceptF)); err != nil {
		return err
	}

	fmt.Println("\n=== 阶段 4: 点名形态对比（SNIPPET 原文是否存在于产物） ===")
	report := similarity.Compare(cur, artifact)
	fmt.Printf("档案 vs 产物 行级相似度(仅供参考，主要看 SNIPPET 点名行): %.3f\n", report.Score())
	for _, s := range doc.Sections {
		if s.Name == "SNIPPET" {
			for _, line := range s.Lines {
				stripped := strings.TrimSpace(line)
				if stripped == "" {
					continue
				}
				if strings.Contains(artifact, stripped) {
					fmt.Printf("  [PASS] SNIPPET 行含于产物: %s\n", stripped)
				} else {
					fmt.Printf("  [FAIL] SNIPPET 行缺失: %s\n", stripped)
				}
			}
		}
	}
	fmt.Printf("\n端到端完成：repo=%s  commits=%d\n", repoDir, lenLog(store))
	return nil
}

func flagSlice(m map[string]string) []string {
	var out []string
	for k, v := range m {
		out = append(out, "--"+k, v)
	}
	return out
}
