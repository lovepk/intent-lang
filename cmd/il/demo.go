package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"intent-lang/internal/agent"
	"intent-lang/internal/archive"
	"intent-lang/internal/il"
	"intent-lang/internal/provider"
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

	// key A builds, key B reproduces. --provider-a/--provider-b allow the two
	// ends to be different models (the cross-model portability check).
	provA, provB := providerName(f), providerName(f)
	if v := strings.TrimSpace(f["provider-a"]); v != "" {
		provA = v
	}
	if v := strings.TrimSpace(f["provider-b"]); v != "" {
		provB = v
	}
	keyA, err := provider.FromEnv(provA, "A")
	if err != nil {
		return err
	}
	keyB, err := provider.FromEnv(provB, "B")
	if err != nil {
		return err
	}
	agA := &agent.Agent{Model: keyA}
	agB := &agent.Agent{Model: keyB}

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
		turn, err := agA.Record(ctx, cur, msg)
		if err != nil {
			fmt.Println("!!", err)
			continue
		}
		if !turn.Changed {
			fmt.Printf("turn: %s\n  reply: %s\n  (档案未变更)\n", msg, turn.Reply)
			continue
		}
		before := cur
		deprecated = agent.NextDeprecated(deprecated, before, turn.Archive)
		summary := archive.Summarize(before, turn.Archive)
		c, err := store.Append(summary, msg, before, turn.Archive, turn.Reply,
			agent.BuildMetaLines(lenLog(store)+1, createdCarry, deprecated))
		if err != nil {
			return err
		}
		cur = c.Archive
		createdCarry, _ = agent.ParseMetaCarry(cur)
		fmt.Printf("turn: %s\n  reply: %s\n  commit %s: %s\n", msg, turn.Reply, c.ID, c.Message)
	}
	if cur == "" {
		return fmt.Errorf("demo: no archive produced")
	}
	archiveOut := filepath.Join(repoDir, "archive.last.il")
	if err := os.WriteFile(archiveOut, []byte(cur), 0o644); err != nil {
		return err
	}

	fmt.Println("\n=== 阶段 2: 删会话，key B 仅凭档案复现 ===")
	doc, err := il.Parse(cur)
	if err != nil {
		return err
	}
	artifact, err := agB.Reproduce(ctx, cur)
	if err != nil {
		return err
	}
	artifactFile := filepath.Join(repoDir, "artifact"+agent.ArtifactExt(doc.HeaderRaw["TARGET"]))
	if err := os.WriteFile(artifactFile, []byte(artifact), 0o644); err != nil {
		return err
	}
	fmt.Printf("复现产物已写入 %s（%d 字节）\n", artifactFile, len(artifact))

	fmt.Println("\n=== 阶段 3: ACCEPT 行为验收 ===")
	acceptF := map[string]string{"archive": archiveOut, "artifact": artifactFile}
	if err := accept(flagSlice(acceptF)); err != nil {
		return err
	}

	fmt.Println("\n=== 阶段 4: 点名形态检查（SNIPPET 原文是否存在于产物） ===")
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
