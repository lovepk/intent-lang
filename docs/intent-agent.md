# Agent 集成契约 (Agent Integration Contract)

> 本文规定：**一个 agent 要"使用 IL"需要遵守什么**。读者是 **agent 实现者**（框架作者、工具/库开发者），不是终端用户。
> 语言本体见 `intent-spec.md`；定位（协议而非产品）见 `intent-goals.md` §2.1–2.2。

---

## 0. 这份契约解决什么

IL 的目标是"其他 agent 能直接消费"。为此需要一份**与具体实现无关**的契约：任意 agent 无需了解本仓库代码，也能正确地**产生 / 消费**档案。本仓库的 CLI 只是这套契约的一个参考实现。

---

## 1. 三种消费面（任选其一接入）

| 消费面 | 做法 | 适合 |
|---|---|---|
| **直接遵守规范** | agent 自己读写 `.il`，用 `intent-spec.md` 的提示词调用自己的 LLM | 无依赖、已有 LLM 的 agent |
| **嵌入库** | 把解析/校验/diff/复现作为库调用 | 自研 agent / 语言内集成 |
| **调用工具接口** | 把 record / reproduce / verify / lint 当工具调用（如 MCP tool） | 通用 agent 宿主 |

> 三个消费面**共享同一套契约**，下面各节对三者都成立。
>
> **参考实现**：`il mcp`（stdio）提供工具面——tools `il_parse` / `il_lint` / `il_diff` / `il_resolve` / `il_read` / `il_commit` / `il_log`；prompts `write` / `repro` / `accept` / `lint` / `normalize`；resources `il://spec` / `il://agent` / `il://keywords`。默认不调用模型（BYOM）。配置了 API key 时，额外提供可选编排工具 `il_record` / `il_verify` / `il_reproduce` / `il_accept` / `il_lint_semantic`（由服务端代调模型）。
>
> **嵌入库**：Go 程序 import `intent-lang/pkg/intentlang`，实现 `intentlang.Model` 接口（BYOM）后即可 `Record`/`Reproduce`/`Accept`，并用 `Parse`/`Lint`/`CompareEntries`/`OpenStore` 等做确定性操作。

---

## 2. 契约 A：档案读写（所有 agent 必须遵守）

- 档案是唯一持久资产，扩展名 `.il`，一份档案 = 一个产物（spec §2.1）。
- **全量重写**：每轮产出**完整**档案，禁止只输出 patch/片段（spec §4.1）。这是"失忆后仅凭 B 可复现"的硬约束。
- **不得输出 META**：META 由宿主维护（spec §3.8）。宿主注入 LLM 前先剥离，收到输出后依据 diff 重建。
- 段结构、编号规则（只增不复用、作废记 `META.deprecated`）按 spec §2–§3。

## 3. 契约 B：双通道输出（写档案的 agent）

- 每轮输出**一个 JSON 对象**，字段顺序硬要求：
  ```json
  {"intent_update": "完整档案文本或空字符串", "reply": "给用户的中文回答"}
  ```
- 与产物无关的消息（闲聊/提问/确认）：`intent_update` 为 `""`。
- 系统提示**原样注入**规范：见 `internal/il/prompt.go` 的 `SpecPrompt`（宿主不得改写其语义）。
- **记录优先（原则 0）**：宿主只记录，**不得**因内容而拒绝、重试、阻断、扣留或改写生成。

## 4. 契约 C：复现（消费档案的 agent）

- 只依赖**自包含**档案：遇到 `<ref: name#entry@ver>` 必须先展开（参考 `internal/refs.Resolve`），喂给 LLM 前不得残留引用。
- 遵守复现指令（`ReproPrompt`）的规则：每条 `R` 必须落实；让所有 `ACCEPT` 成立；`SNIPPET` 逐字迁移；`DECISIONS` 的 reject 不得引入；`OPEN` 按 `default` 处理且交付时列出。
- `FIDELITY` 决定自由空间（`behavior` 自由实现，`artifact` 逼近原形）。

## 5. 契约 D：验收（判定复现是否成功）

- `ACCEPT` 是**可判定的断言**，复现端交付前自检；验收逐条打钩。
- **默认行为一致**；**形态一致**只对 `SNIPPET` 点名的部分生效。不被点名的实现细节允许不同，不算失败。

## 6. 契约 E：引用与版本

- `<ref>` 必须带版本；被引档案更新**不自动**影响引用方，跟进须显式改 `<ref>` 版本（spec §2.6）。
- 引用只允许出现在 `CONTRACT` / `ACCEPT` / `ANCHORS`。
- agent 应声明自己支持的规范版本；档案在 `META.spec` 记录书写所用版本。

---

## 7. 合规自检清单（实现者可逐条打勾）

- [ ] 能解析并生成规范 `.il` 档案
- [ ] 全量重写，不输出 patch
- [ ] 不写 META；注入前剥离 META
- [ ] 双通道 JSON 字段顺序正确（intent_update 在前）
- [ ] 只记录、不干扰（不拒绝/重试/阻断）
- [ ] 复现前展开 `<ref>`，保证自包含
- [ ] `reject` 不引入、`OPEN` 按 `default`
- [ ] 交付前自检全部 `ACCEPT`

---

## 8. 非契约（本文件不管的事）

- 具体传输：MCP / 库 / 直接调用均可，本契约不绑定。
- 跨 agent 的网络协议、权限、账号：明确非目标（见 `intent-goals.md` §5）。
- UI / 产品形态：不做（定位是协议层）。
