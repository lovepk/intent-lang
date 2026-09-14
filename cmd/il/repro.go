package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lovepk/intent-lang/internal/agent"
	"github.com/lovepk/intent-lang/internal/il"
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
	resolved, err := agent.LoadResolvedSource(sourceOpts(f))
	if err != nil {
		return fmt.Errorf("load: %w", err)
	}
	doc, err := il.Parse(resolved)
	if err != nil {
		return fmt.Errorf("archive: %w", err)
	}

	out, err := (&agent.Agent{Model: p}).Reproduce(context.Background(), resolved)
	if err != nil {
		return err
	}

	outFile := f["out"]
	if outFile == "" {
		base := f["name"]
		if archiveFile != "" {
			base = strings.TrimSuffix(filepath.Base(archiveFile), filepath.Ext(archiveFile))
		}
		if base == "" {
			base = "artifact"
		}
		outFile = base + agent.ArtifactExt(doc.HeaderRaw["TARGET"])
	}
	if err := os.WriteFile(outFile, []byte(out), 0o644); err != nil {
		return err
	}
	fmt.Printf("产物已写入 %s（%d 字节，%s）\n", outFile, len(out), p.Name())
	return nil
}
