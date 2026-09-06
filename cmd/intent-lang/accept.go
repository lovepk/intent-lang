package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"intent-lang/internal/il"
	"intent-lang/internal/provider"
)

func accept(args []string) error {
	f := flags(args)
	p, err := makeProvider(f)
	if err != nil {
		return err
	}
	p = retryProvider(p, f)

	archiveFile := f["archive"]
	artifactFile := f["artifact"]
	if archiveFile == "" || artifactFile == "" {
		return fmt.Errorf("accept requires --archive <file.intent> --artifact <artifact>")
	}
	archiveData, err := os.ReadFile(archiveFile)
	if err != nil {
		return err
	}
	absArtifact, err := filepath.Abs(artifactFile)
	if err != nil {
		return err
	}

	ctx := context.Background()
	resp, err := p.Complete(ctx, provider.Request{
		System:  il.AcceptTestPrompt,
		Archive: string(archiveData),
		User:    fmt.Sprintf("产物路径: %s\n请据此生成验收脚本。", absArtifact),
		Mode:    provider.ModeRepro,
	})
	if err != nil {
		return err
	}

	script := provider.StripFence(resp.Reply)
	tmp := filepath.Join(os.TempDir(), "il_accept_test.py")
	if err := os.WriteFile(tmp, []byte(script), 0o644); err != nil {
		return err
	}
	fmt.Println("== 生成的验收脚本 ==\n" + script + "\n")

	out, _ := exec.Command("python", "-X", "utf8", tmp, absArtifact).CombinedOutput()
	fmt.Println("== 验收结果 ==")
	fmt.Print(string(out))
	return nil
}
