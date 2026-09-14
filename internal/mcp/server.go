package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"

	"github.com/lovepk/intent-lang/internal/il"
	"github.com/lovepk/intent-lang/internal/provider"
)

// Server speaks the Model Context Protocol over a newline-delimited JSON-RPC
// 2.0 stream (the stdio transport). It exposes the deterministic half of IL as
// tools/prompts/resources. When a model is configured (WithModel), it also
// exposes the optional orchestrator tools (record/reproduce/accept); without
// one it stays pure and never calls any model (BYOM).
type Server struct {
	in      io.Reader
	out     io.Writer
	repo    string // default repo directory
	name    string // default archive name
	version string
	model   provider.Provider
}

// NewServer builds a server. repo/name are defaults used when a tool call does
// not specify its own.
func NewServer(in io.Reader, out io.Writer, repo, name string) *Server {
	return &Server{in: in, out: out, repo: repo, name: name, version: il.SpecVersion}
}

// WithModel attaches a model and enables the optional orchestrator tools. When
// no model is attached, the server exposes only deterministic tools.
func (s *Server) WithModel(p provider.Provider) *Server {
	s.model = p
	return s
}

// Serve reads requests until EOF and writes responses. Notifications (messages
// without an id) never produce a response.
func (s *Server) Serve() error {
	dec := json.NewDecoder(bufio.NewReader(s.in))
	enc := json.NewEncoder(s.out)
	for {
		var req request
		if err := dec.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		isNotification := len(req.ID) == 0
		result, rerr := s.dispatch(req)
		if isNotification {
			continue
		}
		resp := response{JSONRPC: "2.0", ID: req.ID}
		if rerr != nil {
			resp.Error = rerr
		} else {
			resp.Result = result
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
}

func (s *Server) dispatch(req request) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		return s.initialize(req.Params), nil
	case "notifications/initialized", "notifications/cancelled":
		return nil, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": s.toolList()}, nil
	case "tools/call":
		return s.callTool(req.Params)
	case "resources/list":
		return map[string]any{"resources": s.resourceList()}, nil
	case "resources/read":
		return s.readResource(req.Params)
	case "prompts/list":
		return map[string]any{"prompts": s.promptList()}, nil
	case "prompts/get":
		return s.getPrompt(req.Params)
	default:
		return nil, &rpcError{Code: codeMethodNotFound, Message: "method not found: " + req.Method}
	}
}

func (s *Server) initialize(params json.RawMessage) any {
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(params, &p)
	version := p.ProtocolVersion
	if version == "" {
		version = "2024-11-05"
	}
	return map[string]any{
		"protocolVersion": version,
		"capabilities": map[string]any{
			"tools":     map[string]any{},
			"resources": map[string]any{},
			"prompts":   map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "intent-lang",
			"version": s.version,
		},
		"instructions": s.instructions(),
	}
}

// instructions is the human/agent-readable guidance returned by initialize.
func (s *Server) instructions() string {
	base := "IL 是给 agent 用的意图档案协议。用 prompts/get 取权威提示词（write/repro/accept/lint/normalize），" +
		"用 resources/read 读 il://spec 与 il://agent；确定性操作走 il_parse/il_lint/il_diff/il_resolve/il_read/il_commit/il_log。" +
		"典型 BYOM 流程：取 write 提示词 → 用自己的模型产出完整档案 → il_lint 校验 → il_commit 落库。"
	if s.model != nil {
		base += "本服务已配置模型，也可直接调用编排工具 il_record/il_reproduce/il_accept。"
	} else {
		base += "本服务未配置模型，只提供确定性工具（BYOM）。"
	}
	return base
}

// toolResult renders a tool handler's text output as an MCP tool result.
func toolResult(text string, err error) (any, *rpcError) {
	if err != nil {
		return map[string]any{
			"content": []content{{Type: "text", Text: err.Error()}},
			"isError": true,
		}, nil
	}
	return map[string]any{
		"content": []content{{Type: "text", Text: text}},
	}, nil
}

func badParams(format string, a ...any) *rpcError {
	return &rpcError{Code: codeInvalidParams, Message: fmt.Sprintf(format, a...)}
}
