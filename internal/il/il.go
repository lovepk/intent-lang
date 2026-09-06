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

func (d *Doc) AddSection(name string) *Section {
	s := &Section{Name: name}
	d.Sections = append(d.Sections, s)
	return s
}

func (d *Doc) AddLine(name, line string) {
	if s := d.Section(name); s != nil {
		s.Lines = append(s.Lines, line)
		return
	}
	s := d.AddSection(name)
	s.Lines = []string{line}
}

func (d *Doc) NextID(name string) string {
	re := entryIDRe[name]
	if re == nil {
		return ""
	}
	max := 0
	if s := d.Section(name); s != nil {
		for _, line := range s.Lines {
			if m := re.FindStringSubmatch(line); m != nil {
				if n, ok := s.NumOfID(m[1]); ok && n > max {
					max = n
				}
			}
		}
	}
	prefix := "R"
	switch name {
	case "ACCEPT":
		prefix = "A"
	case "DECISIONS":
		prefix = "D"
	case "OPEN":
		prefix = "?"
	}
	return fmt.Sprintf("%s%d", prefix, max+1)
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

// MetaUnchanged reports whether the META section in newDoc equals the one in
// oldDoc (spec: META is maintained by the Agent, the LLM must not edit it).
func MetaUnchanged(oldDoc, newDoc *Doc) bool {
	oldLines := oldDoc.MetaLines()
	newLines := newDoc.MetaLines()
	if len(oldLines) != len(newLines) {
		return false
	}
	for i := range oldLines {
		if oldLines[i] != newLines[i] {
			return false
		}
	}
	return true
}
