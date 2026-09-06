package main

import (
	"fmt"
	"os"

	"intent-lang/internal/il"
)

func lint(args []string) error {
	f := flags(args)
	archiveFile := f["archive"]
	if archiveFile == "" && len(args) > 0 {
		archiveFile = args[0]
	}
	if archiveFile == "" {
		return fmt.Errorf("lint requires --archive <file.intent>")
	}
	data, err := os.ReadFile(archiveFile)
	if err != nil {
		return err
	}
	doc, err := il.Parse(string(data))
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	errs := doc.Validate()
	if len(errs) > 0 {
		fmt.Printf("语法校验: %d 个硬伤\n", len(errs))
		for _, e := range errs {
			fmt.Println("  [error] " + e.Error())
		}
		fmt.Println("建议先用 Retry 机制让模型修正，或人工修正后再 lint。")
		return nil
	}
	fmt.Println("语法校验: 通过")
	fmt.Print(doc.LintString())
	return nil
}
