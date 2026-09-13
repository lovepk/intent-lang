package main

import (
	"os"

	"intent-lang/internal/mcp"
	"intent-lang/internal/provider"
)

// cmdMCP runs the MCP server over stdio, exposing the deterministic half of IL
// (parse/lint/diff/resolve/read/commit/log) plus prompts and resources to any
// MCP-capable agent. When an API key is present, it also exposes the optional
// orchestrator tools (record/reproduce/accept); otherwise it stays pure and
// never calls a model.
func cmdMCP(args []string) error {
	f := flags(args)
	srv := mcp.NewServer(os.Stdin, os.Stdout, f["repo"], f["name"])
	if keyEnv := envKeyFor(f["key"]); os.Getenv(keyEnv) != "" {
		srv.WithModel(provider.NewDeepSeekFromEnv(keyEnv))
	}
	return srv.Serve()
}
