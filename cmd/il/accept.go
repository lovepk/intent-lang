package main

import (
	"context"
	"fmt"

	"github.com/anomalyco/intent-lang/internal/agent"
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
	archiveText, err := agent.LoadResolvedSource(sourceOpts(f))
	if err != nil {
		return err
	}

	script, output, err := (&agent.Agent{Model: p}).Accept(context.Background(), archiveText, artifactFile)
	if err != nil {
		return err
	}
	fmt.Println("== 生成的验收脚本 ==\n" + script + "\n")
	fmt.Println("== 验收结果 ==")
	fmt.Print(output)
	return nil
}
