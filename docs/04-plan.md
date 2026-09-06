# 计划 (Plan) — 里程碑路线

> 原则：文档先行 → mock 闭环先跑通 → 接口稳定后再接真实 LLM。每个里程碑都有可验证的验收标准。

## M0 — 文档与骨架（已完成）

**产出**
- 目录骨架、`go.mod`。
- 本文档体系：`01-goals` / `02-scenarios` / `03-design` / `il-spec` / `04-plan`。
- `testdata/calculator.intent` 示例档案（用于贯穿后续测试）。

**验收**
- 文档评审通过；计算器示例档案可作为统一测试输入。

## M1 — IL 与 mock Provider 基础（已完成）

**内容**
- `internal/il`：档案的解析（分段、编号抽取）、轻校验、规范化输出（供 diff）。校验覆盖 il-spec §6：头四段齐全、`FIDELITY` 取值合法、`CONTRACT` 编号唯一且递增、`OPEN` 均带 `default:`、正文无空段、正文去重。
- `internal/provider`：`Provider` 接口（Request{System, Archive, User} / Response{Reply, IntentUpdate}）+ `mock` 实现。
- mock 行为：确定性；根据用户消息更新档案并产出 reply；遵守 il-spec §4（全量重写、校验自检）。可 `NewMock(name)` 建多实例。

**验收**
- Go 单元测试通过：il 解析/校验/规范化 + mock 确定性 + 计算器消息序列更新 + 无关消息不产生 commit + 无重复功能。
- 非法档案被 `il` 校验拒绝，并有对应测试。

## M2 — 对话内核与档案仓库（当前）

**内容**
- `internal/agent`：装配（规范 + 最新 B）→ 调 provider → 校验 → 更新 → commit。
- `internal/archive`：commit 链持久化（文件 JSON/文本）、`log/show/rollback/export`。

**验收**
- CLI 能跑场景 1 的第 1–7 步（对话式），打印 reply 与每次 commit（diff + message）。
- 场景 4（续写不依赖历史消息）通过：只给【规范+最新 B+新消息】即可继续。
- 回滚测试通过（场景 3 的 commit 操作）。

## M3 — 复现与相似度

**内容**
- `internal/repro`：给定【规范+B】调 provider 重建 C'。
- `internal/similarity`：C 与 C' 行 diff + 相似度；ACCEPT 用例逐条判定。
- mock 提供两个"不同个性"的实例（LLM-A/LLM-B），供跨模型验证。

**验收**
- 场景 1 完整闭环：删除 session → 【规范+B@vN】→ 复现 → 相似度报告 ≥ 0.95。
- 场景 2：A 写的档案交给 B 复现，报告同样达标。

## M4 — 端到端演示与校准

**内容**
- 一条 `go run` / 脚本演示完整"计算器 golden path"（对应场景 1 全步骤）。
- 校准相似度阈值与报告格式。
- 补齐 README 使用说明。

**验收**
- 新人按 README 可在离线状态复现 demo 与全部报告。

## M5 —（后续，接真实 LLM）

**内容**
- `provider/openai` 等真实实现；上下文注入规范文本；`IntentUpdate` 解析与容错（3 次重试降级）。
- 真实模型下重新跑场景 1/2，观察相似度并据此修订 IL 语法（哪些约束写进 CONTRACT/ANCHORS 才能稳住真实模型）。

**验收**
- 真实 LLM 下"失忆复现"相似度报告产出；记录与 mock 的差异，驱动 `il-spec` 修订。

## 展望（超出当前范围，仅记录）

- 档案库检索（场景 5）；多档案引用；让意图档案被确定性编译器直接消费（脱离 LLM）的探索。
- 若成功，这等于把"对话"变成一种可版本管理、可迁移、可长期持有的资产。

## 风险

| 风险 | 影响 | 缓解 |
|------|------|------|
| LLM 不遵守强模板 | 档案无法解析 | 轻校验 + 重试降级；mock 先固化期望 |
| 真实模型漂移 > 阈值 | 复现相似度不达标 | 用 CONTRACT/ANCHORS/ACCEPT 迭代收紧规范 |
| 全量重写开销 | token 消耗高 | DEC-1 已选；后续可加 patch 能力开关 |
| 人类与 LLM 的意图表述本就模糊 | OPEN 膨胀 | 把"消歧"设计为逐步收敛的显式过程 |
