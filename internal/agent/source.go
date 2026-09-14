package agent

import (
	"fmt"
	"os"

	"github.com/lovepk/intent-lang/internal/archive"
	"github.com/lovepk/intent-lang/internal/il"
	"github.com/lovepk/intent-lang/internal/refs"
)

// SourceOpts selects where an archive comes from: a single .il file, or a
// named archive inside a repo directory.
type SourceOpts struct {
	Archive string // path to a .il file (takes precedence)
	Repo    string // repo directory (default .il)
	Name    string // archive name (default main)
}

// OpenStore opens the archive store described by the options.
func OpenStore(o SourceOpts) (*archive.Store, error) {
	dir := o.Repo
	if dir == "" {
		dir = ".il"
	}
	name := o.Name
	if name == "" {
		name = archive.DefaultName
	}
	return archive.OpenArchive(dir, name)
}

// RepoRefLoader returns a refs.Loader that resolves target archives by name
// from the same repo (reading each archive's latest committed text).
func RepoRefLoader(store *archive.Store) refs.Loader {
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

// LoadSourceRaw loads an archive's latest text WITHOUT expanding refs (the
// source form, for linting/editing).
func LoadSourceRaw(o SourceOpts) (string, error) {
	if o.Archive != "" {
		data, err := os.ReadFile(o.Archive)
		return string(data), err
	}
	store, err := OpenStore(o)
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

// LoadResolvedSource loads an archive's latest text and expands any <ref>
// references into a self-contained archive.
func LoadResolvedSource(o SourceOpts) (string, error) {
	source, err := LoadSourceRaw(o)
	if err != nil {
		return "", err
	}
	return ExpandWithRepo(o, source)
}

// ExpandWithRepo resolves references using a repo-scoped loader when available.
func ExpandWithRepo(o SourceOpts, source string) (string, error) {
	doc, err := il.Parse(source)
	if err != nil {
		return source, err
	}
	if len(doc.ScanRefs()) == 0 {
		return source, nil
	}
	store, err := OpenStore(o)
	if err != nil {
		return "", err
	}
	return refs.Resolve(source, RepoRefLoader(store))
}
