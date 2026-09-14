# Changelog

本项目遵循语义化版本。**规范版本**（`docs/intent-spec.md` 的 `vX.Y`）与**工具/库版本**分开演进：

- **规范**：`minor` 只新增语法元素（旧档案仍合法、仍可解析复现）；`major` 才允许破坏性变更并附迁移说明。
- **工具 / 库**（CLI、`il mcp`、`pkg/intentlang`）：遵循 [SemVer](https://semver.org/)。

## [Unreleased]

### Added
- **多 Provider**：`--provider <name>` 接入任意 OpenAI 兼容端点（OpenAI / Qwen / 本地 Ollama…），key 可空；`demo --provider-a/--provider-b` 跨模型；DeepSeek 环境变量名不变，向后兼容。
- MCP 编排工具补齐 `il_verify`（单轮预览）与 `il_lint_semantic`（LLM 语义复查）。

### Changed
- **开源**：采用 Apache-2.0 许可；module path 改为 `github.com/anomalyco/intent-lang`（外部可直接 `go get`）。
- 新增 `CONTRIBUTING.md`、GitHub Actions CI（离线 build/vet/test）。

## [0.1.0] - 2026-09-13

### Added
- **IL 规范 v2.0**：五条根本原则（原则 0 = 记录优先、不干扰生成）+ 头四段/七段结构 + 跨档案引用 `<ref>`。
- **档案仓库**：commit 链、`log`/`show`/`rollback`、多档案（`--name`）与 `<ref: name#entry@ver>` 引用展开。
- **双通道对话内核**：L1 确定性检查 + L2 记录员（仅 L1 报警时触发）的分层一致性。
- **复现闭环**：`repro` / `accept` / `demo`（ACCEPT 行为验收 + SNIPPET 点名检查）。
- **CLI**：`chat` / `verify` / `repro` / `accept` / `lint` / `fmt` / `demo` / `mcp` / `log` / `show` / `rollback`。
  - `lint --fail-on error`：可作为 CI / pre-commit 门禁。
  - `fmt`：幂等规范化排版。
- **`il mcp`**：MCP server（stdio）。纯工具 `il_parse`/`il_lint`/`il_diff`/`il_resolve`/`il_read`/`il_commit`/`il_log` + prompts + resources；配 API key 时可选编排工具 `il_record`/`il_reproduce`/`il_accept`。默认 BYOM（不调用模型）。
- **`pkg/intentlang`**：公开 Go 嵌入库（BYOM：实现 `Model` 接口即可）。
- **文档**：`intent-agent.md`（Agent 集成契约）、`intent-ecosystem.md`（场景与生态）。
- **编辑器**：VS Code 扩展 + TextMate grammar。
