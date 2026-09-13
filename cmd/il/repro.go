package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

	archiveFile := f["archive"]
	if archiveFile == "" && f["name"] == "" && f["repo"] == "" {
		return fmt.Errorf("repro requires --archive <file.il> 或 --repo <dir> [--name <档案名>]")
	}
	resolved, err := loadResolvedSource(f)
	if err != nil {
		return fmt.Errorf("load: %w", err)
	}

	doc, err := il.Parse(resolved)
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
		base := f["name"]
		if archiveFile != "" {
			base = strings.TrimSuffix(filepath.Base(archiveFile), filepath.Ext(archiveFile))
		}
		if base == "" {
			base = "artifact"
		}
		outFile = base + artifactExt(doc.HeaderRaw["TARGET"])
	}
	if err := os.WriteFile(outFile, []byte(out), 0o644); err != nil {
		return err
	}
	fmt.Printf("产物已写入 %s（%d 字节，%s）\n", outFile, len(out), p.Name())
	return nil
}
