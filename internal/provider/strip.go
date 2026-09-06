package provider

import (
	"regexp"
	"strings"
)

var fenceRe = regexp.MustCompile("(?s)^\\s*```[a-zA-Z0-9_+-]*\\s*\\n?")

// StripFence removes a single leading and trailing markdown code fence if
// present. Used defensively on reproduced artifacts.
func StripFence(s string) string {
	out := s
	if i := fenceRe.FindStringIndex(out); i != nil && i[0] == 0 {
		out = out[i[1]:]
	}
	out = strings.TrimSpace(out)
	if strings.HasSuffix(out, "```") {
		out = strings.TrimSuffix(out, "```")
	}
	return strings.TrimSpace(out)
}
