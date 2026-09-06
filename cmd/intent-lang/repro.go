package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"intent-lang/internal/il"
	"intent-lang/internal/provider"
)

func repro(args []string) error {
	f := flags(args)
	p, err := makeProvider(f)
	if err != nil {
		return err
	}
	p = retryProvider(p, f)

	archiveFile := f["archive"]
	if archiveFile == "" {
		return fmt.Errorf("repro requires --archive <file.intent>")
	}
	data, err := os.ReadFile(archiveFile)
	if err != nil {
		return err
	}

	doc, err := il.Parse(string(data))
	if err != nil {
		return fmt.Errorf("archive: %w", err)
	}
	if errs := doc.Validate(); len(errs) != 0 {
		return fmt.Errorf("archive invalid: %v", errs)
	}

	ctx := context.Background()
	resp, err := p.Complete(ctx, provider.Request{
		System:  il.ReproPrompt,
		Archive: doc.Canonical(),
		User:    "请据此档案重建产物。",
		Mode:    provider.ModeRepro,
	})
	if err != nil {
		return err
	}

	out := strings.TrimSpace(resp.Reply)
	outFile := f["out"]
	if outFile == "" {
		outFile = "artifact.py"
	}
	if err := os.WriteFile(outFile, []byte(out), 0o644); err != nil {
		return err
	}
	fmt.Printf("产物已写入 %s（%d 字节，%s）\n", outFile, len(out), p.Name())
	return nil
}
