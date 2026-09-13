package main

import (
	"fmt"
	"os"

	"intent-lang/internal/archive"
	"intent-lang/internal/il"
	"intent-lang/internal/refs"
)

// repoRefLoader returns a refs.Loader that resolves target archives by name
// from the same repo (reading each archive's latest committed text).
func repoRefLoader(store *archive.Store) refs.Loader {
	return func(name string) (string, bool) {
		target, err := archive.OpenArchive(store.Dir(), name)
		if err != nil {
			return "", false
		}
		latest, err := target.Latest()
		if err != nil || latest == nil {
			return "", false
		}
		return latest.Archive, true
	}
}

// loadSourceRaw loads an archive's latest text WITHOUT expanding refs (the
// source form, for linting/editing). Modes same as loadResolvedSource.
func loadSourceRaw(f map[string]string) (string, error) {
	file := f["archive"]
	if file != "" {
		data, err := os.ReadFile(file)
		return string(data), err
	}
	store, err := openStore(f)
	if err != nil {
		return "", err
	}
	latest, err := store.Latest()
	if err != nil {
		return "", err
	}
	if latest == nil {
		return "", fmt.Errorf("archive %q is empty (no commits yet)", store.Name())
	}
	return latest.Archive, nil
}

// loadResolvedSource loads an archive's latest text and expands any <ref>
// references into a self-contained archive. Two modes:
//
//	--archive <file.il>  : read that single file directly (repo optional)
//	--name <name>        : read <name> from the --repo store (default main)
//
// Returns the resolved (reference-free) archive text.
func loadResolvedSource(f map[string]string) (string, error) {
	source, err := loadSourceRaw(f)
	if err != nil {
		return "", err
	}
	return expandWithRepo(f, source)
}

// expandWithRepo resolves references using repo-scoped loader when available.
func expandWithRepo(f map[string]string, source string) (string, error) {
	doc, err := il.Parse(source)
	if err != nil {
		return source, err
	}
	if len(doc.ScanRefs()) == 0 {
		return source, nil
	}
	store, err := openStore(f)
	if err != nil {
		return "", err
	}
	return refs.Resolve(source, repoRefLoader(store))
}
