# @lovepk/intent-lang

Intent archive format & protocol for agents — **CLI + MCP server**.

This package is a thin distribution wrapper: it installs the platform-specific
`il` binary (published as an optional dependency) and exposes the `il`
command. The implementation lives in the
[Go repository](https://github.com/lovepk/intent-lang); there is no separate JS
implementation to drift.

## Install

```sh
npm install -g @lovepk/intent-lang
# or run without installing:
npx -y @lovepk/intent-lang --help
```

## Usage

```sh
il lint  --archive x.il --fail-on error
il repro --key B --archive x.il --out artifact.py
il mcp   --repo .il            # MCP server over stdio
```

MCP client config (e.g. Claude Desktop):

```json
{
  "mcpServers": {
    "intent-lang": {
      "command": "npx",
      "args": ["-y", "@lovepk/intent-lang", "mcp", "--repo", ".il"]
    }
  }
}
```

Model-driven commands (`chat`/`repro`/`accept`/`demo`, and MCP orchestrator
tools) need a provider configured via environment variables; see the
[CLI docs](https://github.com/lovepk/intent-lang/blob/main/docs/intent-commands.md).
Deterministic commands (`lint`/`fmt`/`parse`/`commit`…) need no key.

## Supported platforms

`linux-x64`, `linux-arm64`, `darwin-x64`, `darwin-arm64`, `win32-x64`,
`win32-arm64`.

## License

Apache-2.0.
