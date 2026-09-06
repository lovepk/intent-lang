package refs

import (
	"fmt"
	"strings"

	"intent-lang/internal/il"
)

// Loader supplies an archive's text by name (without .il extension).
type Loader func(name string) (string, bool)

const maxDepth = 16

// Resolve materializes a source archive into a self-contained archive text:
// every <ref: name#entry@ver> is replaced by the target entry's body (recursively
// resolved), leaving no references behind. Used before feeding any LLM.
func Resolve(sourceText string, load Loader) (string, error) {
	doc, err := il.Parse(sourceText)
	if err != nil {
		return "", err
	}
	visit := map[string]bool{}
	out := &il.Doc{
		HeaderRaw: map[string]string{},
		Header:    doc.Header,
	}
	for k, v := range doc.HeaderRaw {
		out.HeaderRaw[k] = v
	}
	for _, sec := range doc.Sections {
		ns := &il.Section{Name: sec.Name, Label: sec.Label}
		for _, line := range sec.Lines {
			ml, err := resolveLine(line, sec.Name, visit, load, 0)
			if err != nil {
				return "", err
			}
			ns.Lines = append(ns.Lines, ml)
		}
		out.Sections = append(out.Sections, ns)
	}
	return out.Canonical(), nil
}

func resolveLine(line, section string, visit map[string]bool, load Loader, depth int) (string, error) {
	if depth > maxDepth {
		return "", fmt.Errorf("reference nesting too deep (>%d)", maxDepth)
	}
	// Find every <ref> marker in this line and expand in place.
	var b strings.Builder
	rest := line
	for {
		start := strings.Index(rest, "<ref:")
		if start < 0 {
			b.WriteString(rest)
			break
		}
		b.WriteString(rest[:start])
		end := strings.IndexByte(rest[start:], '>')
		if end < 0 {
			return "", fmt.Errorf("unterminated <ref> in line: %q", line)
		}
		end += start + 1
		marker := rest[start:end]
		ref, err := parseMarker(marker)
		if err != nil {
			return "", fmt.Errorf("in %s: %w", line, err)
		}
		body, err := resolveRef(ref, visit, load, depth+1)
		if err != nil {
			return "", err
		}
		b.WriteString(body)
		rest = rest[end:]
	}
	return b.String(), nil
}

func parseMarker(marker string) (il.Ref, error) {
	inner := strings.TrimSuffix(strings.TrimPrefix(marker, "<ref:"), ">")
	inner = strings.TrimSpace(inner)
	// name#entry@version
	at := strings.LastIndex(inner, "@")
	hash := strings.LastIndex(inner, "#")
	if at < 0 || hash < 0 || hash > at {
		return il.Ref{}, fmt.Errorf("bad ref marker %q (want name#entry@version)", marker)
	}
	return il.Ref{
		Archive: inner[:hash],
		Entry:   inner[hash+1 : at],
		Version: inner[at+1:],
	}, nil
}

func resolveRef(ref il.Ref, visit map[string]bool, load Loader, depth int) (string, error) {
	targetText, ok := load(ref.Archive)
	if !ok {
		return "", fmt.Errorf("ref target archive %q not found", ref.Archive)
	}
	targetDoc, err := il.Parse(targetText)
	if err != nil {
		return "", fmt.Errorf("ref target %q unparseable: %w", ref.Archive, err)
	}
	// version + name check against target INTENT (e.g. "common@1.2")
	intent := targetDoc.Header.Intent
	name, ver, ok := splitIntent(intent)
	if !ok {
		return "", fmt.Errorf("ref target %q has malformed INTENT %q", ref.Archive, intent)
	}
	if name != ref.Archive {
		return "", fmt.Errorf("ref target file %q but INTENT declares %q", ref.Archive, name)
	}
	if ver != ref.Version {
		return "", fmt.Errorf("ref %q version mismatch: archive INTENT %q, ref wants @%s",
			ref.Archive, intent, ref.Version)
	}

	// find entry text in the target
	entryText := findEntry(targetDoc, ref.Entry)
	if entryText == "" {
		return "", fmt.Errorf("ref target entry %s not found in %s", ref.Entry, ref.Archive)
	}

	// detect cycles on archive+entry
	key := ref.Archive + "#" + ref.Entry
	if visit[key] {
		return "", fmt.Errorf("circular reference detected at %s", key)
	}
	visit[key] = true
	defer delete(visit, key)

	body := entryBodyOf(entryText)
	// recursively resolve refs inside the pulled entry (target may itself ref)
	section := sectionOfEntry(ref.Entry)
	ml, err := resolveLine(body, section, visit, load, depth+1)
	if err != nil {
		return "", err
	}
	return appendSource(ml, ref), nil
}

// splitIntent splits "name@version" into (name, version).
func splitIntent(intent string) (name, version string, ok bool) {
	i := strings.LastIndex(intent, "@")
	if i <= 0 || i == len(intent)-1 {
		return "", "", false
	}
	return intent[:i], intent[i+1:], true
}

func findEntry(doc *il.Doc, id string) string {
	prefix := string(id[0])
	var section string
	switch prefix {
	case "R":
		section = "CONTRACT"
	case "A":
		section = "ACCEPT"
	case "D":
		section = "DECISIONS"
	default:
		section = "OPEN"
	}
	s := doc.Section(section)
	if s == nil {
		return ""
	}
	for _, line := range s.Lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, id+":") || strings.HasPrefix(t, id+" :") {
			return t
		}
	}
	return ""
}

func sectionOfEntry(id string) string {
	switch string(id[0]) {
	case "R":
		return "CONTRACT"
	case "A":
		return "ACCEPT"
	case "D":
		return "DECISIONS"
	default:
		return "OPEN"
	}
}

func entryBodyOf(entryLine string) string {
	if i := strings.Index(entryLine, ":"); i >= 0 {
		return strings.TrimSpace(entryLine[i+1:])
	}
	return strings.TrimSpace(entryLine)
}

// appendSource appends a provenance comment after a materialized body. The
// marker text intentionally omits "<ref:" so it is never re-scanned as a ref.
func appendSource(body string, ref il.Ref) string {
	return body + " （来源: " + ref.Archive + "#" + ref.Entry + "@" + ref.Version + "）"
}
