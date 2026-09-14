package main

import (
	"fmt"
	"os"

	"github.com/lovepk/intent-lang/internal/il"
)

// fmtCmd normalizes an archive's layout to the canonical form (header order,
// section order, 2-space indent, SNIPPET label). It prints to stdout by
// default; --write rewrites the file in place. It is idempotent.
func fmtCmd(args []string) error {
	f := flags(args)
	file := f["archive"]
	if file == "" {
		file = positionalArg(args)
	}
	if file == "" {
		return fmt.Errorf("fmt requires <file.il> or --archive <file.il>")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	doc, err := il.Parse(string(data))
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	out := doc.Canonical()
	if f["write"] == "true" {
		return os.WriteFile(file, []byte(out), 0o644)
	}
	fmt.Print(out)
	return nil
}
