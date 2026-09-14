package archive

import (
	"sort"
	"strconv"
	"strings"

	"github.com/lovepk/intent-lang/internal/il"
)

// Summarize produces a short commit message from the archive diff between two
// documents. It reuses il.CompareEntries/EntryBodies so the summary and the
// machine diff share one definition of "what changed" (including header fields,
// ANCHORS and SNIPPET). Falls back to a generic message when nothing changed.
func Summarize(before, after string) string {
	d := il.CompareEntries(before, after)
	beforeBodies := il.EntryBodies(before)
	afterBodies := il.EntryBodies(after)

	var changes []string
	for _, id := range d.Added {
		changes = append(changes, "+"+id+" "+short(afterBodies[id]))
	}
	for _, id := range d.Removed {
		changes = append(changes, "-"+id+" "+short(beforeBodies[id]))
	}
	for _, id := range d.Modified {
		changes = append(changes, "~"+id+" "+short(afterBodies[id]))
	}
	if len(changes) == 0 {
		return "update archive"
	}
	sort.Strings(changes)
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

func short(body string) string {
	s := strings.Join(strings.Fields(body), " ")
	if r := []rune(s); len(r) > 40 {
		return string(r[:40]) + "…"
	}
	return s
}
