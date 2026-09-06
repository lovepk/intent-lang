package archive

import (
	"regexp"
	"strconv"
	"strings"

	"intent-lang/internal/il"
)

var entryRe = map[string]*regexp.Regexp{
	"CONTRACT":  regexp.MustCompile(`^R(\d+)`),
	"ACCEPT":    regexp.MustCompile(`^A(\d+)`),
	"DECISIONS": regexp.MustCompile(`^D(\d+)`),
	"OPEN":      regexp.MustCompile(`^\?(\d+)`),
}

// Summarize produces a short commit message from the archive diff between two
// canonical documents. Falls back to a generic message when nothing is found.
func Summarize(before, after string) string {
	var changes []string
	b := index(before)
	a := index(after)

	for name := range entryRe {
		ids := union(b[name], a[name])
		for _, id := range ids {
			oldBody, hadOld := b[name][id]
			newBody, hadNew := a[name][id]
			switch {
			case !hadOld && hadNew:
				changes = append(changes, "+"+short(newBody))
			case hadOld && !hadNew:
				changes = append(changes, "-"+short(oldBody))
			case hadOld && hadNew && oldBody != newBody:
				changes = append(changes, "~"+short(newBody))
			}
		}
	}
	if len(changes) == 0 {
		return "update archive"
	}
	if len(changes) > 5 {
		return strings.Join(changes[:5], "; ") + fmtN("; +%d more", len(changes)-5)
	}
	return strings.Join(changes, "; ")
}

func fmtN(format string, n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Replace(format, "%d", strconv.Itoa(n), 1)
}

func union(x, y map[string]string) []string {
	seen := map[string]bool{}
	var out []string
	for id := range x {
		seen[id] = true
		out = append(out, id)
	}
	for id := range y {
		if !seen[id] {
			out = append(out, id)
		}
	}
	return out
}

func short(body string) string {
	s := entryBody(body)
	if len(s) > 40 {
		return s[:40] + "…"
	}
	return s
}

func entryBody(line string) string {
	if i := strings.IndexByte(line, ':'); i >= 0 {
		return strings.TrimSpace(line[i+1:])
	}
	return strings.TrimSpace(line)
}

func index(canonical string) map[string]map[string]string {
	out := map[string]map[string]string{}
	doc, err := il.Parse(canonical)
	if err != nil {
		return out
	}
	for _, name := range []string{"CONTRACT", "ACCEPT", "DECISIONS", "OPEN"} {
		out[name] = map[string]string{}
		sec := doc.Section(name)
		if sec == nil {
			continue
		}
		re := entryRe[name]
		for _, line := range sec.Lines {
			if m := re.FindStringSubmatch(line); m != nil {
				out[name]["ID"+m[1]] = line
			}
		}
	}
	return out
}
