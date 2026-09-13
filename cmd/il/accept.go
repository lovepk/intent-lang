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

	archiveFile := f["archive"]
	if archiveFile == "" {
		archiveFile = positionalArg(args)
		f["archive"] = archiveFile
	}
	artifactFile := f["artifact"]
	if archiveFile == "" && f["repo"] == "" && f["name"] == "" {
		return fmt.Errorf("accept requires --archive <file.il> 或 --repo <dir> [--name <档案名>]，以及 --artifact <artifact>")
	}
	if artifactFile == "" {
		return fmt.Errorf("accept requires --artifact <artifact>")
	}
	archiveText, err := loadResolvedSource(f)
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
		Archive: archiveText,
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

	// The acceptance harness is a Python script regardless of artifact
	// language; the interpreter can be overridden via IL_PYTHON.
	python := os.Getenv("IL_PYTHON")
	if python == "" {
		python = "python"
	}
	out, _ := exec.Command(python, "-X", "utf8", tmp, absArtifact).CombinedOutput()
	fmt.Println("== 验收结果 ==")
	fmt.Print(string(out))
	return nil
}
