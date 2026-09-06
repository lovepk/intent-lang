package il

import (
	"fmt"
	"regexp"
	"strings"
)

// Ref is a cross-archive entry reference, syntax:
//
//	<ref: <archive>#<entry>@<version>>
//
// e.g. <ref: common#R3@1.2>
type Ref struct {
	Archive string // target archive file name without .il
	Entry   string // target entry id, e.g. "R3"
	Version string // target archive version, e.g. "1.2"
}

// allowedRefSections lists which archive sections may contain <ref> markers.
// CONTRACT/ACCEPT/ANCHORS hold shared facts/cases/examples; DECISIONS/OPEN/META
// are archive-private and must not reference others.
var allowedRefSections = map[string]bool{
	"CONTRACT": true,
	"ACCEPT":   true,
	"ANCHORS":  true,
}

var refRe = regexp.MustCompile(`<ref:\s*([A-Za-z0-9_.\-]+)#([RAD?]\d+)@([A-Za-z0-9_.\-]+)>`)

// LocatedRef carries a found reference plus context.
type LocatedRef struct {
	Ref     Ref
	Section string // owning section name (CONTRACT/ACCEPT/...)
	Entry   string // owning entry id (e.g. "R1") or "" if not in an entry line
	Text    string // full entry/line text containing the ref
	Line    int    // 1-based line index within the owning section
	Offset  int    // byte offset of the ref marker within Text
}

func scanLines(section string, lines []string) []LocatedRef {
	var out []LocatedRef
	for i, line := range lines {
		owner := ""
		trim := strings.TrimSpace(line)
		for _, p := range []string{"R", "A", "D", "?"} {
			if strings.HasPrefix(trim, p) {
				if idx := strings.Index(trim, ":"); idx > 0 {
					owner = strings.TrimSpace(trim[:idx])
				}
				break
			}
		}
		for _, m := range refRe.FindAllStringSubmatchIndex(line, -1) {
			ref := Ref{
				Archive: line[m[2]:m[3]],
				Entry:   line[m[4]:m[5]],
				Version: line[m[6]:m[7]],
			}
			out = append(out, LocatedRef{Ref: ref, Section: section, Entry: owner, Text: line, Line: i + 1, Offset: m[0]})
		}
	}
	return out
}

// ScanRefs scans one section for refs. Returns nil for sections where
// references are not allowed (DECISIONS/OPEN/META) or nil section.
func (s *Section) ScanRefs() []LocatedRef {
	if s == nil || !allowedRefSections[s.Name] {
		return nil
	}
	return scanLines(s.Name, s.Lines)
}

// ScanRefs returns refs found in the reference-allowed sections.
func (d *Doc) ScanRefs() []LocatedRef {
	var out []LocatedRef
	for _, name := range []string{"CONTRACT", "ACCEPT", "ANCHORS"} {
		if s := d.Section(name); s != nil {
			out = append(out, s.ScanRefs()...)
		}
	}
	return out
}

// ScanRefsAll scans every section, including forbidden ones, so linters can
// report references placed in DECISIONS/OPEN/META.
func (d *Doc) ScanRefsAll() []LocatedRef {
	var out []LocatedRef
	for _, s := range d.Sections {
		out = append(out, scanLines(s.Name, s.Lines)...)
	}
	return out
}

// RefAllowed reports whether a section may contain references.
func RefAllowed(sectionName string) bool { return allowedRefSections[sectionName] }

func (r Ref) String() string {
	return fmt.Sprintf("<ref: %s#%s@%s>", r.Archive, r.Entry, r.Version)
}

// RefSyntax is exposed for docs and prompts.
func RefSyntax() string { return "<ref: 档案名#条目ID@版本>" }
