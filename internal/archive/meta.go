package archive

import (
	"github.com/lovepk/intent-lang/internal/il"
)

// attachMeta re-serializes an archive text, stripping any META it may carry
// and appending the agent-controlled meta lines (when provided).
func attachMeta(after string, meta []string) string {
	doc, err := il.Parse(after)
	if err != nil {
		return after
	}
	if len(meta) == 0 {
		return doc.StripMeta().Canonical()
	}
	doc = doc.WithMeta(meta)
	return doc.Canonical()
}
