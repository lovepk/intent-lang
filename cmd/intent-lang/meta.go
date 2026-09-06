package main

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"intent-lang/internal/il"
)

// buildMetaLines builds the META section for the next commit.
// createdCarry is the original creation timestamp (kept across commits);
// deprecated is the accumulated list of removed CONTRACT ids.
func buildMetaLines(commits int, createdCarry string, deprecated []string) []string {
	if createdCarry == "" {
		createdCarry = time.Now().UTC().Format(time.RFC3339)
	}
	return []string{
		il.MetaSpecLine(),
		"created: " + createdCarry,
		"commits: " + strconv.Itoa(commits),
		"deprecated: " + il.FormatDeprecated(deprecated),
	}
}

// parseMetaCarry extracts created timestamp and deprecated list from an
// existing archive text (or "" for none).
func parseMetaCarry(archiveText string) (created string, deprecated []string) {
	if strings.TrimSpace(archiveText) == "" {
		return "", nil
	}
	doc, err := il.Parse(archiveText)
	if err != nil {
		return "", nil
	}
	for _, line := range doc.MetaLines() {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "created:") {
			created = strings.TrimSpace(strings.TrimPrefix(t, "created:"))
		}
	}
	return created, doc.MetaDeprecated()
}

// nextDeprecated returns deprecated + R ids present before but absent after.
func nextDeprecated(existing []string, beforeText, afterText string) []string {
	before := setOf(contractIDsOf(beforeText))
	after := setOf(contractIDsOf(afterText))
	seen := map[string]bool{}
	var out []string
	for _, id := range existing {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for id := range before {
		if !after[id] && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

func contractIDsOf(text string) []string {
	doc, err := il.Parse(text)
	if err != nil {
		return nil
	}
	return doc.ContractIDs()
}

func setOf(ids []string) map[string]bool {
	m := map[string]bool{}
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// artifactExt returns a filename extension hint derived from the archive's
// TARGET, or "" when the target doesn't clearly imply one. Keeps the CLI from
// hard-coding python.
func artifactExt(target string) string {
	t := strings.ToLower(target)
	switch {
	case strings.Contains(t, "python") || strings.Contains(t, "py"):
		return ".py"
	case strings.Contains(t, "go"):
		return ".go"
	case strings.Contains(t, "javascript") || strings.Contains(t, "node") || strings.Contains(t, "js"):
		return ".js"
	case strings.Contains(t, "typescript") || strings.Contains(t, "ts"):
		return ".ts"
	case strings.Contains(t, "rust") || strings.Contains(t, "rs"):
		return ".rs"
	case strings.Contains(t, "ruby") || strings.Contains(t, "rb"):
		return ".rb"
	case strings.Contains(t, "java"):
		return ".java"
	case strings.Contains(t, "shell") || strings.Contains(t, "bash") || strings.Contains(t, "sh"):
		return ".sh"
	case strings.Contains(t, "html"):
		return ".html"
	case strings.Contains(t, "css"):
		return ".css"
	case strings.Contains(t, "markdown") || strings.Contains(t, "md"):
		return ".md"
	case strings.Contains(t, "json"):
		return ".json"
	default:
		return ""
	}
}

// undeclaredChanges returns entry ids that were actually changed but not listed
// in the model's declared_changes (the "越权改动" gate).
func undeclaredChanges(beforeText, afterText string, declared []string) []string {
	d := il.CompareEntries(beforeText, afterText)
	declaredSet := map[string]bool{}
	for _, id := range declared {
		declaredSet[id] = true
	}
	var out []string
	for _, id := range d.Changed() {
		if !declaredSet[id] {
			out = append(out, id)
		}
	}
	return out
}
