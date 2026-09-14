package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anomalyco/intent-lang/internal/archive"
)

func storeFromFlags(args []string) (*archive.Store, error) {
	return openStore(flags(args))
}

// positionalArg returns the first non-flag argument (id etc.), skipping
// "--key value" pairs.
func positionalArg(args []string) string {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "--") {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				i++ // skip the flag's value
			}
			continue
		}
		return a
	}
	return ""
}

func cmdLog(args []string) error {
	store, err := storeFromFlags(args)
	if err != nil {
		return err
	}
	log, err := store.Log()
	if err != nil {
		return err
	}
	if len(log) == 0 {
		fmt.Println("(empty repo)")
		return nil
	}
	for i, c := range log {
		marker := "  "
		if i == 0 {
			marker = "* "
		}
		fmt.Printf("%s%s %s | %s | %s\n", marker, c.ID, c.Time.Format(time.RFC3339), c.Message, c.UserMsg)
	}
	return nil
}

func cmdShow(args []string) error {
	f := flags(args)
	store, err := openStore(f)
	if err != nil {
		return err
	}
	id := f["id"]
	if id == "" {
		id = positionalArg(args)
	}
	if id == "" {
		latest, err := store.Latest()
		if err != nil {
			return err
		}
		if latest == nil {
			return fmt.Errorf("empty repo")
		}
		id = latest.ID
	}
	c, err := store.Get(id)
	if err != nil {
		return err
	}
	fmt.Printf("commit %s\nparent: %s\nmsg: %s\nuser: %s\ntime: %s\n--- archive ---\n%s\n",
		c.ID, c.Parent, c.Message, c.UserMsg, c.Time.Format(time.RFC3339), c.Archive)
	return nil
}

func cmdRollback(args []string) error {
	f := flags(args)
	store, err := openStore(f)
	if err != nil {
		return err
	}
	id := f["id"]
	if id == "" {
		id = positionalArg(args)
	}
	if id == "" {
		return fmt.Errorf("rollback requires <id>")
	}
	c, err := store.Get(id)
	if err != nil {
		return err
	}
	head, err := store.Latest()
	if err != nil {
		return err
	}
	if head != nil && head.ID == c.ID {
		return fmt.Errorf("already at %s", id)
	}
	if err := store.SetHead(c.ID); err != nil {
		return err
	}
	out := filepath.Join(store.ArchiveDir(), "archive.last.il")
	if err := os.WriteFile(out, []byte(c.Archive), 0o644); err != nil {
		return err
	}
	fmt.Printf("已回滚到 %s，最新档案写入 %s\n", c.ID, out)
	return nil
}
