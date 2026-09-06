package il

import (
	"strings"
)

// Diff is the set of entry ids whose bodies changed between two archive texts,
// plus added/removed ids. Bodies are compared in a normalized form (whitespace
// collapsed) so pure formatting churn does not count as a change.
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

func normBody(line string) string {
	return strings.Join(strings.Fields(entryBody(line)), " ")
}

func indexByID(text string) map[string]string {
	m := map[string]string{}
	doc, err := Parse(text)
	if err != nil {
		return m
	}
	for _, name := range []string{"CONTRACT", "ACCEPT", "DECISIONS", "OPEN"} {
		prefix := entryPrefixOf(name)
		s := doc.Section(name)
		if s == nil {
			continue
		}
		for _, e := range s.Entries() {
			id := prefix + e.ID
			m[id] = e.Text
		}
	}
	return m
}

// CompareEntries diffs entry ids/bodies between two archive texts.
func CompareEntries(beforeText, afterText string) Diff {
	before := indexByID(beforeText)
	after := indexByID(afterText)

	var d Diff
	for id, line := range after {
		oldLine, ok := before[id]
		if !ok {
			d.Added = append(d.Added, id)
			continue
		}
		if normBody(oldLine) != normBody(line) {
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
