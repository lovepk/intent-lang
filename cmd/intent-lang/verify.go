package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"intent-lang/internal/il"
	"intent-lang/internal/provider"
)

func verify(args []string) error {
	f := flags(args)
	p, err := makeProvider(f)
	if err != nil {
		return err
	}

	archiveFile := f["archive"]
	msg := f["msg"]
	if msg == "" {
		msg = positionalArg(args)
	}
	if msg == "" {
		return fmt.Errorf("verify requires --msg <用户需求>")
	}

	var archive string
	if archiveFile != "" {
		data, err := os.ReadFile(archiveFile)
		if err != nil {
			return err
		}
		archive = string(data)
	}

	ctx := context.Background()
	resp, err := p.Complete(ctx, provider.Request{System: il.SpecPrompt, Archive: archive, User: msg})
	if err != nil {
		return err
	}

	fmt.Println("== reply ==")
	fmt.Println(resp.Reply)
	fmt.Println("== intent_update ==")
	if strings.TrimSpace(resp.IntentUpdate) == "" {
		fmt.Println("(empty — provider判定档案未变更)")
		return nil
	}
	fmt.Println(resp.IntentUpdate)
	return nil
}
