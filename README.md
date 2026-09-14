# intent-lang

[![CI](https://github.com/lovepk/intent-lang/actions/workflows/ci.yml/badge.svg)](https://github.com/lovepk/intent-lang/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/lovepk/intent-lang.svg)](https://pkg.go.dev/github.com/lovepk/intent-lang)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/tag/lovepk/intent-lang?label=release)](https://github.com/lovepk/intent-lang/releases)

一种给 LLM 使用的意图语言 (Intent Language) 及其 agent 工作流：让"人与 LLM 的对话"沉淀为一份**独立于会话、可移植、可复现的意图档案**。

> **定位**：面向 agent 的**意图交换格式 + 记录协议**，不是具体产品。**规范才是产品**，本仓库 CLI 只是参考实现与测试床；其他 agent 通过接口（MCP tool / 库 / 直接遵守双通道协议）消费它。详见 `docs/intent-goals.md` §2.1–2.2。

## 核心思想

```
会话中（人机都记得）
  用户 ──消息──► agent ──► LLM ──► ① 正常回答
                                    ② 意图档案 B 更新（像程序员改代码）
  经过 n 次交流 → 产物 C 完成，档案 B@vN 完成

会话删除（人机双失忆）
  [IL 规范 + 档案 B@vN] ──► 任意 LLM ──► 重建意图一致的 C'
     行为一致（ACCEPT 验收）为默认要求；形态一致只对档案点名锁定的部分生效
```

一切依赖如下四层：

- **意图档案 B**：用 IL 编写的自包含规格，是唯一的持久资产。
- **IL 规范**：注入给 LLM 的语言规则（见 `docs/intent-spec.md`）。
- **双通道协议**：LLM 每次既回答用户，又像改代码一样产出新一版 B。
- **复现闭环**：删掉会话后，仅凭【规范 + B】即可重建产物，用 ACCEPT 行为验收 + SNIPPET 点名检查验收。

## 文档

| 文档 | 内容 |
|------|------|
| `docs/intent-goals.md` | 问题、目标、非目标、术语、成功判据 |
| `docs/intent-commands.md` | **CLI 命令参考**：9 个命令用法/参数/工作流 |
| `docs/intent-spec.md` | 意图语言规范 v2.0（给 LLM 读的完整语言定义） |
| `docs/intent-keywords.md` | **关键词速查手册（给人类看）**：一词一例、一页总览 |
| `docs/intent-ecosystem.md` | 应用场景与生态：能用在哪些场景、如何建立生态 |
| `docs/intent-agent.md` | **Agent 集成契约**：其他 agent 接入 IL 必须遵守的规则与自检清单 |

## 状态

- M0 文档骨架 ✅ / M1 IL ✅ / M2 档案仓库+对话内核 ✅ / M3 复现 ✅ / M4 端到端演示 ✅ / M5 真实 LLM 闭环验证 ✅ / M6 规范打磨 ✅ / M7 多档案引用 ✅ / M8 分层一致性 ✅。
- 技术栈：Go。Provider：默认 `deepseek`；任意 OpenAI 兼容端点（OpenAI/Qwen/本地 Ollama…）用 `--provider <name>` 接入，key 可空（见 `docs/intent-commands.md`「Provider 配置」）。
- 运行需 `.env` 提供 `DEEPSEEK_API_KEY_A/B`（文件已 gitignore，不入库）。

## 用法

```sh
go run ./cmd/il chat  --key A --repo .il
    # 交互式双通道对话：输入需求 → reply + 档案增量更新，每次变更写一条 commit；
    # 只记录不干扰：L1 确定性检查每轮跑，L2 记录员仅在 L1 报警时整理成合法自洽档案；
    # 每轮打印机器 diff 权威摘要；退出打印统计
go run ./cmd/il verify --key A --msg "加上乘法"
    # 单轮遵守度检查：看模型对这份档案的重写是否合法
go run ./cmd/il repro  --key B --archive .il/archive.last.il --out artifact.py
    # 失忆复现：仅凭档案重建产物
go run ./cmd/il accept --archive x.il --artifact artifact.py
    # ACCEPT 行为验收：LLM 将验收用例翻译为可执行测试并运行（输出 PASS/FAIL/TOTAL）
go run ./cmd/il demo --script turns.txt --repo .il-demo
    # 端到端 golden path：key A 建档 → 删会话 → key B 复现 → ACCEPT 验收 → SNIPPET 点名检查
go run ./cmd/il lint --archive x.il            # 确定性结构检查（快/零成本）
go run ./cmd/il lint --llm --key A --archive x.il   # + LLM 单轮语义复查（找需理解力的矛盾，领域无关）
go run ./cmd/il lint --archive x.il --fail-on error # 有 error 时非零退出（CI / pre-commit 门禁）
go run ./cmd/il fmt x.il --write               # 规范化排版（幂等）
go run ./cmd/il log / show <id> / rollback <id> --repo .il
    # 档案版本管理：提交历史 / 查看某 commit / 回滚 HEAD
go run ./cmd/il mcp --repo .il
    # 启动 MCP server（stdio）：把 parse/lint/diff/resolve/read/commit/log 作为工具，
    # 规范/契约作为资源，各角色提示词作为 prompt，暴露给任意 MCP agent；
    # 默认不调用模型（BYOM），调用方自带 LLM；配了 API key 时另提供
    # il_record/il_verify/il_reproduce/il_accept/il_lint_semantic 编排工具
```

CLI 按 `--key A|B` 切换两个凭据充当 LLM-A / LLM-B。`accept`/`demo` 需要本机装有 `python`（验收脚本以标准库运行产物）。

## 作为库嵌入 / MCP 集成

其他 agent 有三种接入方式（都默认 BYOM——模型由调用方自带）：

- **Go 库**：`import "github.com/lovepk/intent-lang/pkg/intentlang"`，实现 `intentlang.Model` 接口后即可 `Record`/`Reproduce`/`Accept`，并用 `Parse`/`Lint`/`CompareEntries`/`OpenStore` 做确定性操作。见 `docs/intent-agent.md`。
- **MCP server**：`il mcp` 把确定性能力与权威提示词暴露给任意 MCP agent（详见 `docs/intent-commands.md` §8）。
- **npm（无需 Go）**：`@lovepk/intent-lang` 分发平台二进制，`npx` 即用。见 `npm/`。

### npm 安装（Node 生态，无需 Go）

```sh
npx -y @lovepk/intent-lang --help
npx -y @lovepk/intent-lang lint --archive x.il --fail-on error
```

MCP 客户端可直接用 `npx` 拉起：

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

接入 MCP 客户端（以 Claude Desktop 的 `claude_desktop_config.json` 为例，用本地二进制）：

```sh
go build -o intent-lang.exe ./cmd/il   # 或 go install ./cmd/il
```

```json
{
  "mcpServers": {
    "intent-lang": {
      "command": "intent-lang",
      "args": ["mcp", "--repo", "E:/path/to/.il"]
    }
  }
}
```

不配 provider → 只暴露确定性工具（纯 BYOM）；配了 provider（如 `DEEPSEEK_API_KEY_A`，或显式 `*_BASE_URL`）→ 额外暴露 `il_record`/`il_verify`/`il_reproduce`/`il_accept`/`il_lint_semantic`。

## 文件格式与编辑器支持

- 档案文件后缀：**`.il`**（规范见 `docs/intent-spec.md` §2.1）。
- **VS Code**：扩展在 `editor/vscode-il/`。打包 + 安装：
  ```sh
  cd editor/vscode-il && npx @vscode/vsce package --out ../intent-lang-il.vsix
  code --install-extension editor/intent-lang-il.vsix
  ```
  或装好后在 VS Code 里 `Ctrl+Shift+P` → `Developer: Reload Window` 生效。注意：**不能用 `code --install-extension <文件夹>` 直接装文件夹**（grammar 需在包内，务必先打包成 .vsix）。
- 没有 VS Code 的编辑器可用通用 TextMate grammar：`editor/vscode-il/syntaxes/intent-language.tmLanguage.json`（scope `source.intent`，Sublime Text / nova / 其它 TextMate 兼容编辑器可直接引用）。
- 高亮覆盖：头部字段（INTENT/KIND/FIDELITY/TARGET）、段落名、条目编号（R/A/D/?）、行内标签（reject:/due:/default:/in:/out:/style:）、`->`、`#` 注释、SNIPPET 原文。

## 验证证据

- `testdata/calculator_llm_verified.il`：真实 deepseek 三轮对话产出的档案；`testdata/artifact_verified.py`：key B 仅凭该档案重建的产物，ACCEPT 黑盒用例全数通过。
- `testdata/demo_calc_archive.il`：7 轮真实对话产出的档案；`testdata/demo_calc_artifact.py`：key B 失忆复现产物，ACCEPT 行为验收 10/10 通过。

## 路线

M0 文档 → M1 IL → M2 档案仓库+对话内核 → M3 复现 → M4 端到端演示 → M5 真实 LLM → M6 规范打磨 → M7 多档案引用 → M8 分层一致性。

生态（见 `docs/intent-ecosystem.md`）：近期务实项已完成——多 Provider（证明可移植）、`il fmt`、`il lint --fail-on` 门禁、规范版本纪律；注册表/LSP/平台等暂缓。

## 许可与贡献

- 开源许可：[Apache License 2.0](LICENSE)。
- 贡献指南：[CONTRIBUTING.md](CONTRIBUTING.md)（构建/测试、规范变更流程、PR 要求）。
- 规范版本与兼容规则见 `docs/intent-spec.md` §9；变更记录见 [CHANGELOG.md](CHANGELOG.md)。
