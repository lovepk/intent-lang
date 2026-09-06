# intent-lang

一种给 LLM 使用的意图语言 (Intent Language) 及其 agent 工作流：让"人与 LLM 的对话"沉淀为一份**独立于会话、可移植、可复现的意图档案**。

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
- **IL 规范**：注入给 LLM 的语言规则（见 `docs/il-spec.md`）。
- **双通道协议**：LLM 每次既回答用户，又像改代码一样产出新一版 B。
- **复现闭环**：删掉会话后，仅凭【规范 + B】即可重建产物，用 ACCEPT 行为验收 + 文本对比报告双轨验收。

## 文档

| 文档 | 内容 |
|------|------|
| `docs/01-goals.md` | 问题、目标、非目标、术语、成功判据 |
| `docs/02-scenarios.md` | 核心场景（计算器 golden path、跨模型、回滚等） |
| `docs/03-design.md` | 架构、数据流、Provider 抽象、边界处理 |
| `docs/04-plan.md` | 里程碑 M0–M6、验收标准、风险 |
| `docs/05-commands.md` | **CLI 命令参考**：10 个命令用法/参数/工作流 |
| `docs/il-spec.md` | 意图语言规范 v2.0（给 LLM 读的完整语言定义） |
| `docs/il-keywords.md` | **关键词速查手册（给人类看）**：一词一例、一页总览 |
| `docs/04-plan.md` | 里程碑 M0–M5、验收标准、风险 |

## 状态

- M0 文档骨架 ✅ / M1 IL ✅ / M2 档案仓库+对话内核 ✅ / M3 复现+相似度 ✅ / M4 端到端演示 ✅ / M5 真实 LLM 闭环验证 ✅（详见 `docs/04-plan.md`）。
- 技术栈：Go。Provider：`deepseek`（真实，OpenAI 兼容协议，可扩展其他模型）。
- 运行需 `.env` 提供 `DEEPSEEK_API_KEY_A/B`（文件已 gitignore，不入库）。

## 用法

```sh
go run ./cmd/il chat  --key A --repo .il
    # 交互式双通道对话：输入需求 → reply + 档案增量更新，每次变更写一条 commit；
    # 退出时打印规范遵守统计（validate 拒绝类型 + lint 位置计数，供语言 v2 演进参考）
go run ./cmd/il verify --key A --msg "加上乘法"
    # 单轮遵守度检查：看模型对这份档案的重写是否合法
go run ./cmd/il repro  --key B --archive .il/archive.last.il --out artifact.py
    # 失忆复现：仅凭档案重建产物
go run ./cmd/il accept --archive x.il --artifact artifact.py
    # ACCEPT 行为验收：LLM 将验收用例翻译为可执行测试并运行（输出 PASS/FAIL/TOTAL）
go run ./cmd/il demo --script turns.txt --repo .il-demo
    # 端到端 golden path：key A 建档 → 删会话 → key B 复现 → ACCEPT 验收 → SNIPPET 点名检查
go run ./cmd/il compare --ref a.py --cand b.py --threshold 0.95 --fidelity structure
    # 形态对比（LCS diff + 分数；behavior 层改用 ACCEPT 行为验收）
go run ./cmd/il lint --archive x.il            # 确定性结构检查（快/零成本）
go run ./cmd/il lint --llm --key A --archive x.il   # + LLM 单轮语义复查（找需理解力的矛盾，领域无关）
go run ./cmd/il log / show <id> / rollback <id> --repo .il
    # 档案版本管理：提交历史 / 查看某 commit / 回滚 HEAD
```

CLI 按 `--key A|B` 切换两个凭据充当 LLM-A / LLM-B。非法档案输出自动带纠正指令重试（`--retry N`，默认 2）。`accept`/`demo` 需要本机装有 `python`（验收脚本以标准库运行产物）。

## 文件格式与编辑器支持

- 档案文件后缀：**`.il`**（规范见 `docs/il-spec.md` §2.1）。
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
- `testdata/demo_calc_archive.il`：7 轮真实对话产出的档案（`docs/04-plan.md` M4 demo）；`testdata/demo_calc_artifact.py`：key B 失忆复现产物，ACCEPT 行为验收 10/10 通过。

## 路线

M0 文档 → M1 IL → M2 档案仓库+对话内核 → M3 复现+相似度 → M4 端到端演示 → M5 真实 LLM。
