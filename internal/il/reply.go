package il

import (
	"fmt"
	"regexp"
	"strings"
)

// ReplyFinding is a deterministic diagnostic about whether a free-text reply
// is consistent with the archive it claims to describe. These are the
// mechanically checkable part of the reply↔archive pair; the semantic part is
// left to the LLM reviewer (see LintReplyPrompt).
type ReplyFinding struct {
	Severity string // "error" (phantom id) | "suggestion" (undeclared change claim)
	ID       string
	Msg      string
}

// replyIDRe extracts entry ids (R1/A2/D3/?4) mentioned in free text. \b is an
// ASCII word boundary, which works because the surrounding prose is non-ASCII;
// it avoids matching ids embedded in larger alphanumerics.
var replyIDRe = regexp.MustCompile(`\b[RAD][0-9]+\b|\?[0-9]+`)

// changeVerbRe is a language-level (not domain-level) heuristic for spotting
// change claims so that an undeclared-change can be suggested.
var changeVerbRe = regexp.MustCompile(`新增|添加|增加|删除|移除|去掉|修改|改为|改成|更新|加入|设为|调整`)

// CheckReplyClaims cross-checks a free-text reply against the archive it
// accompanies:
//
//   - every entry id mentioned in the reply must exist in the archive (error —
//     a hallucinated id);
//   - an id mentioned in the same sentence as a change verb should appear in
//     declared_changes (suggestion — the model may be claiming an undeclared
//     change).
//
// It is intentionally conservative: it only reports what it can decide
// mechanically and never blocks.
func CheckReplyClaims(reply, archiveText string, declared []string) []ReplyFinding {
	if strings.TrimSpace(reply) == "" {
		return nil
	}
	doc, err := Parse(archiveText)
	if err != nil {
		return nil
	}
	exists := allEntryIDs(doc)
	declaredSet := map[string]bool{}
	for _, d := range declared {
		declaredSet[d] = true
	}

	var out []ReplyFinding
	seen := map[string]bool{}
	for _, id := range replyIDRe.FindAllString(reply, -1) {
		if seen[id] {
			continue
		}
		seen[id] = true
		if !exists[id] {
			out = append(out, ReplyFinding{
				Severity: "error",
				ID:       id,
				Msg:      fmt.Sprintf("reply 提到 %s，但档案中不存在该条目（疑似幻觉）", id),
			})
			continue
		}
		if !declaredSet[id] && claimedChange(reply, id) {
			out = append(out, ReplyFinding{
				Severity: "suggestion",
				ID:       id,
				Msg:      fmt.Sprintf("reply 似乎在声称改动了 %s，但 declared_changes 未包含它", id),
			})
		}
	}
	return out
}

// allEntryIDs returns every R/A/D/? id present in the archive.
func allEntryIDs(doc *Doc) map[string]bool {
	m := map[string]bool{}
	for _, name := range []string{"CONTRACT", "ACCEPT", "DECISIONS", "OPEN"} {
		s := doc.Section(name)
		if s == nil {
			continue
		}
		prefix := entryPrefixOf(name)
		for _, e := range s.Entries() {
			m[prefix+e.ID] = true
		}
	}
	return m
}

// claimedChange reports whether id appears in a sentence containing a change verb.
func claimedChange(reply, id string) bool {
	for _, seg := range splitSentences(reply) {
		if strings.Contains(seg, id) && changeVerbRe.MatchString(seg) {
			return true
		}
	}
	return false
}

func splitSentences(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		switch r {
		case '。', '！', '？', '；', '\n', '.', '!', '?', ';':
			return true
		}
		return false
	})
}
