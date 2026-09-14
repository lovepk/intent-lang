package mcp

import (
	"encoding/json"

	"github.com/lovepk/intent-lang/internal/il"
)

// prompt is one MCP prompt: the authoritative system prompt for a role.
type prompt struct {
	name        string
	description string
	text        string
}

func (s *Server) prompts() []prompt {
	return []prompt{
		{"write", "双通道建档端系统提示：模型回答用户的同时更新意图档案。", il.SpecPrompt},
		{"repro", "复现端系统提示：仅凭档案重建产物。", il.ReproPrompt},
		{"accept", "验收脚本生成端系统提示：把 ACCEPT 用例翻译成可执行测试。", il.AcceptTestPrompt},
		{"lint", "语义复查端系统提示：找档案内部的语义矛盾。", il.LintPrompt},
		{"normalize", "记录员系统提示：把草稿整理成合法自洽档案，不改事实。", il.NormalizePrompt},
	}
}

func (s *Server) promptList() []map[string]any {
	out := make([]map[string]any, 0, len(s.prompts()))
	for _, p := range s.prompts() {
		out = append(out, map[string]any{
			"name":        p.name,
			"description": p.description,
		})
	}
	return out
}

func (s *Server) getPrompt(params json.RawMessage) (any, *rpcError) {
	var p struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, badParams("invalid prompts/get params: %v", err)
	}
	for _, pr := range s.prompts() {
		if pr.name == p.Name {
			return map[string]any{
				"description": pr.description,
				"messages": []map[string]any{
					{"role": "user", "content": content{Type: "text", Text: pr.text}},
				},
			}, nil
		}
	}
	return nil, badParams("unknown prompt: %s", p.Name)
}
