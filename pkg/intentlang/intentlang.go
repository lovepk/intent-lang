// Package intentlang is the public embedding API for IL (the intent language).
//
// It lets a Go agent use IL with its own model (BYOM): parse/lint/diff archives,
// resolve cross-references, store versions, and run the dual-channel record /
// reproduce / accept operations. The CLI and the MCP server are built on the
// same internal packages; this package is the stable surface for other agents.
package intentlang

import (
	"context"
	"time"

	"intent-lang/internal/agent"
	"intent-lang/internal/archive"
	"intent-lang/internal/il"
	"intent-lang/internal/provider"
	"intent-lang/internal/refs"
)

// --- model (BYOM) ---

// Mode selects what the model is being asked to do.
type Mode string

const (
	// ModeChat is the dual-channel turn: the model answers the user and updates
	// the archive in one JSON object ({"intent_update","reply"}).
	ModeChat Mode = ""
	// ModeRepro asks the model to rebuild the product from an archive alone.
	ModeRepro Mode = "repro"
	// ModeNormalize asks the model to turn a draft into a valid archive.
	ModeNormalize Mode = "normalize"
	// ModeLint asks the model to review an archive for semantic contradictions.
	ModeLint Mode = "lint"
)

// Request is what the Agent asks the model to complete.
type Request struct {
	System  string
	Archive string
	User    string
	Mode    Mode
}

// Finding is a semantic lint finding produced by a model reviewer.
type Finding struct {
	Severity string
	Section  string
	ID       string
	Msg      string
	Fix      string
}

// Response is the model's raw completion.
type Response struct {
	Reply        string
	IntentUpdate string
	Findings     []Finding
}

// Model is the BYOM interface: bring any LLM that speaks this signature.
type Model interface {
	Complete(ctx context.Context, req Request) (Response, error)
	Name() string
}

// modelAdapter bridges the public Model to the internal provider interface.
type modelAdapter struct{ m Model }

func (a modelAdapter) Name() string { return a.m.Name() }

func (a modelAdapter) Complete(ctx context.Context, req provider.Request) (provider.Response, error) {
	resp, err := a.m.Complete(ctx, Request{
		System:  req.System,
		Archive: req.Archive,
		User:    req.User,
		Mode:    Mode(req.Mode),
	})
	if err != nil {
		return provider.Response{}, err
	}
	out := provider.Response{Reply: resp.Reply, IntentUpdate: resp.IntentUpdate}
	for _, f := range resp.Findings {
		out.Findings = append(out.Findings, provider.Finding{
			Severity: f.Severity, Section: f.Section, ID: f.ID, Msg: f.Msg, Fix: f.Fix,
		})
	}
	return out, nil
}

// --- agent ---

// Agent runs the model-driven half of IL.
type Agent struct{ a *agent.Agent }

// New builds an Agent around a model.
func New(model Model) *Agent {
	return &Agent{a: &agent.Agent{Model: modelAdapter{model}}}
}

// Turn is the outcome of one dual-channel record turn.
type Turn struct {
	Reply      string
	Archive    string // full archive text after the update (no META)
	Changed    bool   // false when the message produced no archive change
	Normalized bool   // true when the L2 recorder rewrote a bad draft
	Diff       Change
}

// Record runs one dual-channel turn: the model answers and updates the archive.
func (a *Agent) Record(ctx context.Context, beforeRaw, userMsg string) (Turn, error) {
	t, err := a.a.Record(ctx, beforeRaw, userMsg)
	if err != nil {
		return Turn{}, err
	}
	return Turn{
		Reply:      t.Reply,
		Archive:    t.Archive,
		Changed:    t.Changed,
		Normalized: t.Normalized,
		Diff:       toChange(t.Diff),
	}, nil
}

// Reproduce rebuilds the product from a self-contained archive.
func (a *Agent) Reproduce(ctx context.Context, resolvedArchive string) (string, error) {
	return a.a.Reproduce(ctx, resolvedArchive)
}

// Accept generates an acceptance script from the archive and runs it against
// the artifact, returning the script and the harness output.
func (a *Agent) Accept(ctx context.Context, resolvedArchive, artifactPath string) (script, output string, err error) {
	return a.a.Accept(ctx, resolvedArchive, artifactPath)
}

// LintLLM runs the semantic lint layer and returns its findings.
func (a *Agent) LintLLM(ctx context.Context, archive string) ([]Finding, error) {
	fs, err := a.a.LintLLM(ctx, archive)
	if err != nil {
		return nil, err
	}
	out := make([]Finding, 0, len(fs))
	for _, f := range fs {
		out = append(out, Finding{Severity: f.Severity, Section: f.Section, ID: f.ID, Msg: f.Msg, Fix: f.Fix})
	}
	return out, nil
}

// NoMeta returns the canonical form of text without the META section.
func NoMeta(text string) string { return agent.NoMeta(text) }

// --- deterministic archive operations ---

// Header is an archive's four head fields.
type Header struct {
	Intent   string
	Kind     string
	Fidelity string
	Target   string
}

// Doc is a parsed archive.
type Doc struct{ d *il.Doc }

// Parse parses an archive text.
func Parse(text string) (*Doc, error) {
	d, err := il.Parse(text)
	if err != nil {
		return nil, err
	}
	return &Doc{d: d}, nil
}

// Canonical renders the archive in canonical form.
func (d *Doc) Canonical() string { return d.d.Canonical() }

// Validate returns structural hard errors.
func (d *Doc) Validate() []error { return d.d.Validate() }

// Header returns the four head fields.
func (d *Doc) Header() Header {
	return Header{
		Intent:   d.d.Header.Intent,
		Kind:     d.d.Header.Kind,
		Fidelity: d.d.Header.Fidelity,
		Target:   d.d.Header.Target,
	}
}

// ContractIDs returns the R<n> ids present in CONTRACT.
func (d *Doc) ContractIDs() []string { return d.d.ContractIDs() }

// Diagnostic is a deterministic consistency finding.
type Diagnostic struct {
	Severity string
	Section  string
	ID       string
	Msg      string
	Fix      string
}

// Lint runs the deterministic consistency checks.
func (d *Doc) Lint() []Diagnostic {
	ds := d.d.Lint()
	out := make([]Diagnostic, 0, len(ds))
	for _, x := range ds {
		out = append(out, Diagnostic{
			Severity: string(x.Severity), Section: x.Section, ID: x.ID, Msg: x.Msg, Fix: x.Fix,
		})
	}
	return out
}

// Change is the set of changed units between two archives.
type Change struct {
	Added    []string
	Removed  []string
	Modified []string
}

// Changed returns all changed unit ids.
func (c Change) Changed() []string {
	out := make([]string, 0, len(c.Added)+len(c.Removed)+len(c.Modified))
	out = append(out, c.Added...)
	out = append(out, c.Removed...)
	out = append(out, c.Modified...)
	return out
}

func toChange(d il.Diff) Change {
	return Change{Added: d.Added, Removed: d.Removed, Modified: d.Modified}
}

// CompareEntries diffs two archive texts.
func CompareEntries(before, after string) Change {
	return toChange(il.CompareEntries(before, after))
}

// Loader supplies an archive's text by name (without extension).
type Loader func(name string) (string, bool)

// Resolve materializes a source archive into a self-contained one by expanding
// every <ref: name#entry@ver>.
func Resolve(text string, load Loader) (string, error) {
	return refs.Resolve(text, refs.Loader(load))
}

// --- repo / source ---

// SourceOpts selects an archive: a .il file, or a named archive in a repo.
type SourceOpts struct {
	Archive string
	Repo    string
	Name    string
}

func toSource(o SourceOpts) agent.SourceOpts {
	return agent.SourceOpts{Archive: o.Archive, Repo: o.Repo, Name: o.Name}
}

// LoadSourceRaw loads an archive's latest text without expanding refs.
func LoadSourceRaw(o SourceOpts) (string, error) { return agent.LoadSourceRaw(toSource(o)) }

// LoadResolvedSource loads an archive's latest text with refs expanded.
func LoadResolvedSource(o SourceOpts) (string, error) {
	return agent.LoadResolvedSource(toSource(o))
}

// ExpandWithRepo resolves refs in source using the repo named by o.
func ExpandWithRepo(o SourceOpts, source string) (string, error) {
	return agent.ExpandWithRepo(toSource(o), source)
}

// Store is a versioned archive repository.
type Store struct{ s *archive.Store }

// OpenStore opens (creating if needed) the archive store.
func OpenStore(dir, name string) (*Store, error) {
	s, err := agent.OpenStore(agent.SourceOpts{Repo: dir, Name: name})
	if err != nil {
		return nil, err
	}
	return &Store{s: s}, nil
}

// Commit is one stored archive version.
type Commit struct {
	ID      string
	Parent  string
	Message string
	UserMsg string
	Archive string
	Replied string
	Time    time.Time
}

func toCommit(c *archive.Commit) *Commit {
	if c == nil {
		return nil
	}
	return &Commit{
		ID: c.ID, Parent: c.Parent, Message: c.Message, UserMsg: c.UserMsg,
		Archive: c.Archive, Replied: c.Replied, Time: c.Time,
	}
}

// Latest returns the HEAD commit, or nil when the store is empty.
func (s *Store) Latest() (*Commit, error) {
	c, err := s.s.Latest()
	return toCommit(c), err
}

// Commit appends a new version. before/after are archive texts; meta is the
// META section (see BuildMetaLines).
func (s *Store) Commit(msg, userMsg, before, after, reply string, meta []string) (*Commit, error) {
	c, err := s.s.Append(msg, userMsg, before, after, reply, meta)
	return toCommit(c), err
}

// Log returns commits from HEAD to the root.
func (s *Store) Log() ([]*Commit, error) {
	cs, err := s.s.Log()
	if err != nil {
		return nil, err
	}
	out := make([]*Commit, 0, len(cs))
	for _, c := range cs {
		out = append(out, toCommit(c))
	}
	return out, nil
}

// RepoRefLoader returns a Loader that resolves archives by name from the same
// repo as s.
func (s *Store) RepoRefLoader() Loader {
	l := agent.RepoRefLoader(s.s)
	return Loader(l)
}

// --- meta helpers ---

// BuildMetaLines builds the META section for a commit.
func BuildMetaLines(commits int, created string, deprecated []string) []string {
	return agent.BuildMetaLines(commits, created, deprecated)
}

// ParseMetaCarry extracts the created timestamp and deprecated list.
func ParseMetaCarry(text string) (created string, deprecated []string) {
	return agent.ParseMetaCarry(text)
}

// NextDeprecated accumulates removed CONTRACT ids.
func NextDeprecated(existing []string, before, after string) []string {
	return agent.NextDeprecated(existing, before, after)
}

// ArtifactExt returns a filename extension hint derived from TARGET.
func ArtifactExt(target string) string { return agent.ArtifactExt(target) }
