package mcp

import (
	"encoding/json"

	"github.com/anomalyco/intent-lang/docs"
)

// resource is one MCP resource: an authoritative read-only document.
type resource struct {
	uri      string
	name     string
	mimeType string
	text     string
}

func (s *Server) resources() []resource {
	return []resource{
		{"il://spec", "IL 意图语言规范 v2.0", "text/markdown", docs.IntentSpec},
		{"il://agent", "Agent 集成契约", "text/markdown", docs.IntentAgent},
		{"il://keywords", "IL 关键词速查（人类）", "text/markdown", docs.IntentKeywords},
	}
}

func (s *Server) resourceList() []map[string]any {
	out := make([]map[string]any, 0, len(s.resources()))
	for _, r := range s.resources() {
		out = append(out, map[string]any{
			"uri":         r.uri,
			"name":        r.name,
			"mimeType":    r.mimeType,
			"description": r.name,
		})
	}
	return out
}

func (s *Server) readResource(params json.RawMessage) (any, *rpcError) {
	var p struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, badParams("invalid resources/read params: %v", err)
	}
	for _, r := range s.resources() {
		if r.uri == p.URI {
			return map[string]any{
				"contents": []map[string]any{
					{"uri": r.uri, "mimeType": r.mimeType, "text": r.text},
				},
			}, nil
		}
	}
	return nil, badParams("unknown resource: %s", p.URI)
}
