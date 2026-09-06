# 计划 (Plan) — 里程碑路线

> 原则：文档先行 → 直接以真实 LLM（DeepSeek）验证闭环 → 接口稳定后扩展更多模型。每个里程碑都有可验证的验收标准。
> 注：早期曾以 mock 跑通流程，已按"全部真实场景测试"决策删除 mock（`mock.go`/`NewMock`/`--provider mock` 均移除），测试仅覆盖不依赖网络的纯逻辑层（il/archive）。

## M0 — 文档与骨架（已完成）

**产出**
- 目录骨架、`go.mod`。
- 本文档体系：`01-goals` / `02-scenarios` / `03-design` / `il-spec` / `04-plan`。
- `testdata/calculator.intent` 示例档案（用于贯穿后续测试）。

**验收**
- 文档评审通过；计算器示例档案可作为统一测试输入。

## M1 — IL 基础与 Provider 接口（已完成）

**内容**
- `internal/il`：档案的解析（分段、编号抽取）、轻校验、规范化输出（供 diff）。校验覆盖 il-spec §6：头四段齐全、`FIDELITY` 取值合法、`CONTRACT` 编号唯一且递增、`OPEN` 均带 `default:`、正文无空段、正文去重。
- `internal/provider`：`Provider` 接口（Request{System, Archive, User, Mode} / Response{Reply, IntentUpdate}）。

**验收**
- Go 单元测试通过：il 解析/校验/规范化（真实 LLM 档案 fixture 必须全过）。
- 非法档案被 `il` 校验拒绝，并有对应测试。
- 说明：早期曾实现计算器 mock 用于离线跑通，后续已删除。

## M2 — 对话内核与档案仓库（已完成）

**内容**
- `internal/archive`：线性 commit 链仓库，持久化为目录（`HEAD` + `c-<ts>.json`）。支持 `Append`（含 commit message 自动摘要 diff）、`Log`、`Show`、`SetHead` 回滚。
- META 归 Agent：注入给 LLM 的档案剥离 META，禁止 LLM 输出 META，收到后 Agent 依 diff 重建（真实验证驱动）。
- CLI 扩展：`chat` 接入仓库（repo 默认 `.intent`，每次档案变更写 commit + 自动 commit message）；新增 `log` / `show` / `rollback <id>`。
- 辅助：`provider.Retry` 装饰器（非法输出追加纠正指令重试）、`provider.StripFence`（repro 产物围栏防御清洗）。

**验收**
- archive 单测：append/head/persist/summarize/rollback 全通过。
- 端到端（真实 LLM）：3 轮 chat → 3 commits 入 `.intent` → `log` 显示自动摘要 → `rollback` 到 c-1 → repro 重建产物反映历史状态。
- META：真实验证中模型篡改 META 被 Agent 接管流程杜绝。

## M3 — 复现与相似度（当前）

**内容**
- `internal/repro`：给定【规范+B】调 provider 重建 C'。
- `internal/similarity`：C 与 C' 行 diff + 相似度；ACCEPT 用例逐条判定。
- 跨模型验证用两个真实凭据（`--key A/B`）作为 LLM-A / LLM-B。

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

## M5 — 真实 LLM 接入（进行中，已提前验证核心闭环）

**已完成**
- `provider/deepseek.go`：DeepSeek（OpenAI 兼容）Provider；`Mode=write` 时强制 JSON 双通道输出，`Mode=repro` 时输出纯产物；返回前对 intent_update 做 il 解析+校验+规范化。
- 密钥经 `.env`（已 gitignore）承载，经 `cmd/intent-lang/dotenv.go` 加载，不入库。
- 真实闭环验证（deepseek-chat，两个 key 充当 A/B 两模型）：
  1. **双通道写档案**：3 轮对话（建加减 → 加乘 → 加除/除零处理），模型只读【当前档案+需求】，增量重写档案正确：编号递增 R1→R6、ACCEPT 同步、DECISIONS 记录否决、OPEN 更新 default、META.commits 递增。全程无需历史消息 → 验证"档案即记忆"。
  2. **失忆复现**：删会话后用 key B 仅凭最终档案重建出可运行的 Python 计算器；ACCEPT 黑盒用例全数通过（含负号解析、除零提示、非法输入、多运算符拒绝、向下取整遵守 OPEN.default）。
  3. **artifact 层 GUI 复现**：key A 建 tkinter 计算器档案（FIDELITY structure，SNIPPET 布局）→ key B 重建 3733 字节产物，SNIPPET 按钮矩阵逐字一致、无幻觉除法、左到右求值。
- 证据归档：`testdata/calculator_llm_verified.intent`（真实 LLM 档案）、`testdata/artifact_verified.py`（复现产物）；`il` 增加回归测试强制真实档案必须通过校验。
- CLI：`chat`（交互双通道）/ `verify`（单轮遵守度检查）/ `repro`（档案→产物重建）。
- 稳定性设施：`provider.Retry`（非法 JSON/档案输出带纠正指令重试，默认 2 次）、`provider.StripFence`（复现产物围栏清洗）。
- **META 归 Agent**（真实验证驱动）：注入给 LLM 的档案剥离 META、模型被禁止输出 META、收到后由 Agent 依据 diff 重建 META（spec/created/commits/deprecated）。原因：真实模型会篡改 META（把 commits 改 0）。

**实测发现（驱动 il-spec 修订）**
1. 模型遵守强模板良好；会把不明确点主动放入 OPEN 并给 default（设计意图被模型理解）。
2. **无档案时模型会幻觉历史**（第二次验证未带 archive，模型杜撰 commits:7 与不存在的先加减后乘）——档案为空时必须显式引导从零建档，不能装作有续写。
3. 复现产物偶尔带 ```python 围栏 → ReproPrompt 已加"不得用围栏包裹"，并配 `StripFence` 防御。
4. SNIPPET 内代码缩进被档案规范化重整为统一 2 空格（无碍 Python 但需留意其他语言）。
5. OPEN.default 会被复现端忠实遵守（5/2 取整），符合设计。
6. **DECISIONS/OPEN 发散**：模型会把单条 D/? 写成几百字"内心独白"推演 → SpecPrompt 与 il-spec 新增"每条一句话、禁止思考流"约束。
7. **模型只加不改**时仍可能幻觉用户没要求的功能 → SpecPrompt 新增"只实现用户明确要求的"。

**待办**
- `internal/similarity`：复现相似度报告（M3）。
- META.deprecated 的 diff 自动计算（当前模型直接删条目，Agent 尚不反写 deprecated 列表）。

## 展望（超出当前范围，仅记录）

- 档案库检索（场景 5）；多档案引用；让意图档案被确定性编译器直接消费（脱离 LLM）的探索。
- 若成功，这等于把"对话"变成一种可版本管理、可迁移、可长期持有的资产。

## 风险

| 风险 | 影响 | 缓解 |
|------|------|------|
| LLM 不遵守强模板 | 档案无法解析 | 轻校验 + `Retry` 重试降级（带纠正指令） |
| 真实模型漂移 > 阈值 | 复现相似度不达标 | 用 CONTRACT/ANCHORS/ACCEPT 迭代收紧规范 |
| 全量重写开销 | token 消耗高 | DEC-1 已选；后续可加 patch 能力开关 |
| 人类与 LLM 的意图表述本就模糊 | OPEN 膨胀 | 把"消歧"设计为逐步收敛的显式过程 |
