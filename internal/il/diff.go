package il

import (
	"strconv"
	"strings"
)

// Diff is the set of change-unit ids whose bodies changed between two archive
// texts, plus added/removed ids. Bodies are compared in a normalized form
// (whitespace collapsed) so pure formatting churn does not count as a change.
type Diff struct {
	Added    []string
	Removed  []string
	Modified []string
}

func (d Diff) Changed() []string {
	var out []string
	out = append(out, d.Added...)
	out = append(out, d.Removed...)
	out = append(out, d.Modified...)
	return out
}

// EntryBodies maps every "change unit" of an archive to its body text. It is
// the single source of truth for machine diffs, so the machine diff and commit
// summaries cannot drift apart. Units:
//
//   - R/A/D/? entries in CONTRACT/ACCEPT/DECISIONS/OPEN;
//   - the four header fields: INTENT / KIND / FIDELITY / TARGET;
//   - the whole ANCHORS section (id "ANCHORS");
//   - each SNIPPET section, keyed "SNIPPET:<label>".
//
// META is deliberately excluded (Agent-owned, not LLM-visible).
func EntryBodies(text string) map[string]string {
	m := map[string]string{}
	doc, err := Parse(text)
	if err != nil {
		return m
	}
	for _, k := range []string{"INTENT", "KIND", "FIDELITY", "TARGET"} {
		if v := doc.HeaderRaw[k]; v != "" {
			m[k] = v
		}
	}
	for _, name := range []string{"CONTRACT", "ACCEPT", "DECISIONS", "OPEN"} {
		s := doc.Section(name)
		if s == nil {
			continue
		}
		prefix := entryPrefixOf(name)
		for _, e := range s.Entries() {
			m[prefix+e.ID] = entryBody(e.Text)
		}
	}
	if s := doc.Section("ANCHORS"); s != nil && len(s.Lines) > 0 {
		m["ANCHORS"] = strings.Join(s.Lines, "\n")
	}
	unnamed := 0
	for _, s := range doc.Sections {
		if s.Name != "SNIPPET" {
			continue
		}
		label := s.Label
		if label == "" {
			unnamed++
			label = "unnamed-" + strconv.Itoa(unnamed)
		}
		m["SNIPPET:"+label] = strings.Join(s.Lines, "\n")
	}
	return m
}

func norm(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// CompareEntries diffs change units between two archive texts. It covers the
// header fields, ANCHORS and SNIPPET in addition to R/A/D/? entries, so silent
// edits outside CONTRACT/ACCEPT/DECISIONS/OPEN are no longer invisible.
func CompareEntries(beforeText, afterText string) Diff {
	before := EntryBodies(beforeText)
	after := EntryBodies(afterText)

	var d Diff
	for id, body := range after {
		old, ok := before[id]
		if !ok {
			d.Added = append(d.Added, id)
			continue
		}
		if norm(old) != norm(body) {
			d.Modified = append(d.Modified, id)
		}
	}
	for id := range before {
		if _, ok := after[id]; !ok {
			d.Removed = append(d.Removed, id)
		}
	}
	return d
}
