package il

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type Header struct {
	Intent   string
	Kind     string
	Fidelity string
	Target   string
}

// SpecVersion is the current IL language specification version. Archives
// record it in META.spec; prompts reference it for LLM expectations.
// v2.0 adds cross-archive references (<ref>).
const SpecVersion = "v2.0"

// MetaSpecLine returns the canonical "spec: <version>" line.
func MetaSpecLine() string { return "spec: " + SpecVersion }

func (h Header) Complete() bool {
	return h.Intent != "" && h.Kind != "" && h.Fidelity != "" && h.Target != ""
}

func ValidFidelity(f string) bool {
	switch f {
	case "behavior", "structure", "artifact":
		return true
	}
	return false
}

type Entry struct {
	ID   string
	Text string
}

type Section struct {
	Name  string
	Label string
	Lines []string
}

func (s *Section) Entries() []Entry {
	re := entryIDRe[s.Name]
	if re == nil {
		return nil
	}
	var out []Entry
	for _, line := range s.Lines {
		if m := re.FindStringSubmatch(line); m != nil {
			out = append(out, Entry{ID: m[1], Text: line})
		}
	}
	return out
}

func (s *Section) NumOfID(id string) (int, bool) {
	var n int
	if _, err := fmt.Sscanf(id, "%d", &n); err != nil {
		return 0, false
	}
	return n, true
}

type Doc struct {
	Header    Header
	HeaderRaw map[string]string
	Sections  []*Section
}

var sectionOrder = []string{"CONTRACT", "ANCHORS", "SNIPPET", "ACCEPT", "DECISIONS", "OPEN", "META"}

var headerKeys = map[string]bool{
	"INTENT": true, "KIND": true, "FIDELITY": true, "TARGET": true,
}

var sectionNames = map[string]bool{
	"CONTRACT": true, "ANCHORS": true, "SNIPPET": true,
	"ACCEPT": true, "DECISIONS": true, "OPEN": true, "META": true,
}

var sectionRe = regexp.MustCompile(`^([A-Z][A-Z ]+)(?:\s|:|$)`)

var entryIDRe = map[string]*regexp.Regexp{
	"CONTRACT":  regexp.MustCompile(`^R(\d+)`),
	"ACCEPT":    regexp.MustCompile(`^A(\d+)`),
	"DECISIONS": regexp.MustCompile(`^D(\d+)`),
	"OPEN":      regexp.MustCompile(`^\?(\d+)`),
}

func sectionKey(line string) string {
	m := sectionRe.FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	key := strings.TrimSpace(m[1])
	if headerKeys[key] || sectionNames[key] {
		return key
	}
	return ""
}

func Parse(text string) (*Doc, error) {
	doc := &Doc{HeaderRaw: map[string]string{}}
	var cur *Section
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Only flush-left (unindented) lines may start a new section or header.
		// Indented lines are always content — this keeps SNIPPET bodies (which
		// may contain lines that look like keywords) from being split.
		flushLeft := line[0] != ' ' && line[0] != '\t'
		if flushLeft {
			if key := sectionKey(trimmed); key != "" {
				switch key {
				case "INTENT", "KIND", "FIDELITY", "TARGET":
					doc.HeaderRaw[key] = headerValue(trimmed)
					continue
				default:
					cur = &Section{Name: key}
					if key == "SNIPPET" {
						cur.Label = headerValue(trimmed)
					}
					doc.Sections = append(doc.Sections, cur)
					continue
				}
			}
		}
		if cur == nil {
			return nil, fmt.Errorf("content before any section header: %q", trimmed)
		}
		cur.Lines = append(cur.Lines, trimmed)
	}
	doc.Header = Header{
		Intent:   doc.HeaderRaw["INTENT"],
		Kind:     doc.HeaderRaw["KIND"],
		Fidelity: doc.HeaderRaw["FIDELITY"],
		Target:   doc.HeaderRaw["TARGET"],
	}
	return doc, nil
}

func headerValue(line string) string {
	rest := line
	if i := strings.IndexByte(line, ' '); i >= 0 {
		rest = strings.TrimSpace(line[i+1:])
	}
	rest = strings.TrimPrefix(rest, ":")
	return strings.TrimSpace(rest)
}

func entryBody(line string) string {
	if i := strings.IndexByte(line, ':'); i >= 0 {
		return strings.TrimSpace(line[i+1:])
	}
	return line
}

func (d *Doc) Section(name string) *Section {
	for _, s := range d.Sections {
		if s.Name == name {
			return s
		}
	}
	return nil
}

type Problem struct {
	Section string
	ID      string
	Msg     string
}

func (p Problem) Error() string {
	switch {
	case p.Section != "" && p.ID != "":
		return fmt.Sprintf("%s %s: %s", p.Section, p.ID, p.Msg)
	case p.Section != "":
		return fmt.Sprintf("%s: %s", p.Section, p.Msg)
	default:
		return p.Msg
	}
}

func (d *Doc) Validate() []error {
	var errs []error
	for _, k := range []string{"INTENT", "KIND", "FIDELITY", "TARGET"} {
		if d.HeaderRaw[k] == "" {
			errs = append(errs, Problem{Msg: "missing header " + k})
		}
	}
	if d.Header.Fidelity != "" && !ValidFidelity(d.Header.Fidelity) {
		errs = append(errs, Problem{Msg: "invalid FIDELITY " + d.Header.Fidelity})
	}

	seen := map[string]bool{}
	for _, s := range d.Sections {
		if seen[s.Name] {
			errs = append(errs, Problem{Section: s.Name, Msg: "duplicate section"})
		}
		seen[s.Name] = true
		if len(s.Lines) == 0 {
			errs = append(errs, Problem{Section: s.Name, Msg: "empty section"})
		}
	}

	for _, name := range []string{"OPEN"} {
		if s := d.Section(name); s != nil {
			for _, line := range s.Lines {
				if !strings.Contains(line, "default:") {
					errs = append(errs, Problem{Section: name, Msg: fmt.Sprintf("missing default: -> %q", line)})
				}
			}
		}
	}

	for _, name := range []string{"CONTRACT", "ACCEPT", "DECISIONS"} {
		s := d.Section(name)
		if s == nil {
			continue
		}
		prev := -1
		for _, e := range s.Entries() {
			n, ok := s.NumOfID(e.ID)
			if !ok {
				errs = append(errs, Problem{Section: name, ID: e.ID, Msg: "bad id"})
				continue
			}
			if n <= prev {
				errs = append(errs, Problem{Section: name, ID: e.ID, Msg: fmt.Sprintf("id not increasing after %d", prev)})
			}
			prev = n
		}
	}

	if s := d.Section("CONTRACT"); s != nil {
		textSeen := map[string]string{}
		for _, e := range s.Entries() {
			body := entryBody(e.Text)
			if id, dup := textSeen[body]; dup {
				errs = append(errs, Problem{Section: "CONTRACT", ID: e.ID, Msg: fmt.Sprintf("duplicate body as %s", id)})
			}
			textSeen[body] = e.ID
		}
	}

	return errs
}

func (d *Doc) Canonical() string {
	var b strings.Builder
	writeHeader := func(key string) {
		if v := d.HeaderRaw[key]; v != "" {
			fmt.Fprintf(&b, "%s %s\n", key, v)
		}
	}
	writeHeader("INTENT")
	writeHeader("KIND")
	writeHeader("FIDELITY")
	writeHeader("TARGET")

	order := map[string]int{}
	for i, n := range sectionOrder {
		order[n] = i
	}
	sections := append([]*Section(nil), d.Sections...)
	sort.SliceStable(sections, func(i, j int) bool {
		return order[sections[i].Name] < order[sections[j].Name]
	})

	for _, sec := range sections {
		fmt.Fprintf(&b, "%s", sec.Name)
		if sec.Name == "SNIPPET" && sec.Label != "" {
			fmt.Fprintf(&b, " %s", sec.Label)
		}
		b.WriteString("\n")
		for _, line := range sec.Lines {
			fmt.Fprintf(&b, "  %s\n", line)
		}
	}
	return b.String()
}

func (d *Doc) String() string { return d.Canonical() }

// MetaLines returns the raw META section lines, or nil when absent.
func (d *Doc) MetaLines() []string {
	if s := d.Section("META"); s != nil {
		return s.Lines
	}
	return nil
}

// StripMeta returns a copy of the doc without the META section.
func (d *Doc) StripMeta() *Doc {
	out := &Doc{
		Header:    d.Header,
		HeaderRaw: map[string]string{},
	}
	for k, v := range d.HeaderRaw {
		out.HeaderRaw[k] = v
	}
	for _, s := range d.Sections {
		if s.Name == "META" {
			continue
		}
		cp := &Section{Name: s.Name, Label: s.Label, Lines: append([]string(nil), s.Lines...)}
		out.Sections = append(out.Sections, cp)
	}
	return out
}

// WithMeta appends (or replaces) a META section with the given lines.
func (d *Doc) WithMeta(lines []string) *Doc {
	out := d.StripMeta()
	out.Sections = append(out.Sections, &Section{Name: "META", Lines: append([]string(nil), lines...)})
	return out
}

// MetaDeprecated parses the META.deprecated list (format: [R3, R7] or R3, R7).
func (d *Doc) MetaDeprecated() []string {
	for _, line := range d.MetaLines() {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "deprecated:") {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(t, "deprecated:"))
		val = strings.Trim(val, "[]")
		if val == "" {
			return nil
		}
		parts := strings.Split(val, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			s := strings.TrimSpace(p)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// ContractIDs returns the set of R<n> ids present in CONTRACT.
func (d *Doc) ContractIDs() []string {
	s := d.Section("CONTRACT")
	if s == nil {
		return nil
	}
	var out []string
	for _, e := range s.Entries() {
		out = append(out, "R"+e.ID)
	}
	return out
}

// FormatDeprecated renders a deprecated list as a META line value, e.g. [R3, R7].
func FormatDeprecated(list []string) string {
	var parts []string
	for _, s := range list {
		if s == "" {
			continue
		}
		parts = append(parts, strings.TrimPrefix(s, "R"))
	}
	if len(parts) == 0 {
		return "[]"
	}
	return "[R" + strings.Join(parts, ", R") + "]"
}
