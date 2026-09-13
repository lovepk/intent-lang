# 计划 (Plan) — 里程碑路线

> 原则：文档先行 → 直接以真实 LLM（DeepSeek）验证闭环 → 接口稳定后扩展更多模型。每个里程碑都有可验证的验收标准。
> 注：早期曾以 mock 跑通流程，已按"全部真实场景测试"决策删除 mock（`mock.go`/`NewMock`/`--provider mock` 均移除），测试仅覆盖不依赖网络的纯逻辑层（il/archive）。

## M0 — 文档与骨架（已完成）

**产出**
- 目录骨架、`go.mod`。
- 本文档体系：`01-goals` / `02-scenarios` / `03-design` / `il-spec` / `04-plan`。
- `testdata/calculator.il` 示例档案（用于贯穿后续测试）。

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
- CLI 扩展：`chat` 接入仓库（repo 默认 `.il`，每次档案变更写 commit + 自动 commit message）；新增 `log` / `show` / `rollback <id>`。
- 辅助：`provider.Retry` 装饰器（非法输出追加纠正指令重试）、`provider.StripFence`（repro 产物围栏防御清洗）。

**验收**
- archive 单测：append/head/persist/summarize/rollback 全通过。
- 端到端（真实 LLM）：3 轮 chat → 3 commits 入 `.il` → `log` 显示自动摘要 → `rollback` 到 c-1 → repro 重建产物反映历史状态。
- META：真实验证中模型篡改 META 被 Agent 接管流程杜绝。

## M3 — 复现与相似度（已完成）

**内容**
- `internal/similarity`：行级 LCS diff + 归一化相似度报告（ref/cand 行数、公共行、hunks、`Pass(threshold)`）。
- CLI `compare`：`compare --ref a --cand b --threshold 0.95 [--fidelity behavior|structure]`。`behavior` 层仅提示改用 ACCEPT 行为验收，不做文本阈值判定。
- 跨模型验证用两个真实凭据（`--key A/B`）作为 LLM-A / LLM-B。
- ReproPrompt 强化：SNIPPET 必须【原样逐字】出现（不得改写/重排），只允许外围最小胶水补全。

**实测结论（重要设计证据）**
1. **behavior 层文本相似度无意义**：同一档案让 key A/B 各复现 CLI 计算器，文本相似度仅 0.127，但两者的 ACCEPT 黑盒用例全部通过。→ behavior 层应验收"行为"，不是文本。
2. **structure/artifact 层才有文本对比意义**：key A/B 复现 GUI 计算器文本 0.17~0.18（各自独立实现）。
3. **SNIPPET 强约束生效**：ReproPrompt 改为"原样逐字"后，key B 产物开始逐字包含档案 SNIPPET 的 `buttons=[...]` 矩阵与 `display.grid(...)` 行——这是 artifact 保真的机制。
4. **对比对象**：两个独立复现会话的文本天然不同；有意义的对比是"产物 vs 档案 SNIPPET/ACCEPT"，而非"产物 A vs 产物 B"。ACCEPT 行为验收是最可靠的一致性判据。

**验收**
- similarity 单测通过（identical=1.0、disjoint=0、partial 区间、Pass 阈值）。
- 真实双 key 对比实验完成并归档结论如上。

## M4 — 端到端演示与校准（已完成）

**内容**
- `accept` 命令：ACCEPT 自动行为验收器——LLM 把档案 ACCEPT 用例翻译成可执行测试脚本（每个用例独立启动产物做子串断言），本机运行输出 PASS/FAIL/TOTAL。
- `demo` 命令：全自动 golden path——key A 从脚本文件多轮建档 → 删会话 → key B 仅凭档案复现 → ACCEPT 验收 → SNIPPET 点名行存在性检查。
- 校准与实测发现（重要）：
  1. **子进程编码**：Windows 下产物中文输出按 GBK，测试端需 `encoding='utf-8', errors='replace'` + `PYTHONIOENCODING=utf-8`，否则 UnicodeDecodeError 让 stdout=None。
  2. **规格矛盾会被验收器抓到**：点名锁定 `.strip()` 的结构与"含空格非法"契约冲突时，产物选 strip 导致 A7 失败——验收器正确暴露了档案自相矛盾（demo_turns 已修正为"内部空格非法"）。
  3. 生成的测试脚本质量是验收可信度的关键，AcceptTestPrompt 经多轮迭代（独立进程/子串匹配/编码/try-except 包裹）后稳定。

**验收**
- 端到端 demo 全绿：7 轮建档（7 commits）→ key B 复现 → ACCEPT **10/10 通过**（含 `= 8` 格式自适应）；SNIPPET 点名行 `while True: expr = input('> ').strip()` 完整出现在复现产物。
- 证据归档：`testdata/demo_calc_archive.il`（7 轮真实档案）、`testdata/demo_calc_artifact.py`（key B 复现产物）。

## M6 — 语言规范打磨到 v2（第一批完成；LLM 单轮语义复查定案）

> 目标：语言从"能用"走向"可靠、自洽、可演进"。方向=语言规范优先（用户拍板），暂不追服务形态。

**Lint 架构定案（用户决策）**：lint 分两层——
1. **确定性结构检查** `il.Lint`（快/零成本/领域无关）：只做针对 IL 语言本身的结构规则。
2. **LLM 单轮语义复查** `lint --llm`（`ModeLint` + `LintPrompt`）：理解任意领域词，找需要语义的矛盾。
- **明确不做**：绑死"算子词表"的领域规则（已删除）；不做 agent 循环（用户拍板先保持单轮）。

**已完成**
- `il.Lint` 精简为通用结构规则：SNIPPET 存在但 FIDELITY=behavior（suggestion）、FIDELITY=artifact 却无 SNIPPET（error）、META.spec 版本不一致（suggestion）、OPEN 的 default 为空（suggestion）。
- 删除原领域性规则（CONTRACT 否定 vs ACCEPT、reject vs CONTRACT 的算子词表匹配）——那是对"计算器领域"的偷懒实现，非 IL 语言本身的检查。
- `LintPrompt`：LLM 语义复查指令。含 severity 判定准则：只有"复现端无法同时满足两条规则"才标 error；措辞不同但实现可兼容 → suggestion。多轮实测校准。
- `provider.ModeLint`：LLM 以 json_object 输出 `{"findings":[...]}`。
- `il.SpecVersion`/`il.MetaSpecLine()` 统一版本来源；`ComplianceStats` 遵守率统计（chat 退出打印）。
- **SpecPrompt 语义收紧**：DECISIONS.reject 只写"本次被否定的备选"，禁止写"改动前旧状态"（实测 calculator_llm_verified D1 因此误冲突）。il-keywords 加 ⚠️。

**实测验证（LLM 单轮语义复查）**
- **非计算器领域抓真矛盾**：人为构造订单系统档案（R3 不做图片 vs R1 可传图 vs A1 测 jpg vs D1 reject）→ 复查精确报出 3 处 error 并给改法。证明领域无关通用性。
- **抓到 ACCEPT 运行时验收查不出的契约矛盾**：demo_calc_archive 的 R5（输出"错误：格式应为…"）与 R7/ACCEPT A4-A8（输出"非法输入"）存在历史遗留的两套错误消息冲突——产物实现统一成"非法输入"所以 ACCEPT 全过，但契约层确实不自洽。复查正确报出（这是产物验收的盲区，两条链路互补）。
- 干净的订单档案（无矛盾）复查应零发现；severity 分级经校准（措辞不一致降 suggestion）。

**低级设计错误全量审计与修复（领域无关性收尾）**
1. 🔴 **SNIPPET 内容会被段名截断**（解析器文法缺陷）：修复=段头必须顶格，缩进内容永不开启新段；SNIPPET 原文规范格式缩进存储。补 il-spec §2.4 排版文法。单测覆盖（缩进关键词行不再误判）。
2. 🟠 **META.deprecated 恒为空**（规范-实现不符）：Agent 现依据 before/after 的 CONTRACT 编号差集计算 deprecated 并跨 commit 累积；created 保留首次值。`cmd/meta.go` + 单测。实测发现 LLM 多倾向"改写条目"而非删除，deprecated 主要在真删条目时触发。
3. 🟡 **accept/repro/demo 硬编码 Python**：repro/demo 按 TARGET 推断产物扩展名（`artifactExt`）；repro 默认名从 archive 文件名推；accept 解释器可经 `IL_PYTHON` 覆盖。
4. 🟡 **reject 语义只改了一端**：ReproPrompt 消费侧补齐（reject=备选非旧状态；与 CONTRACT 冲突按 reject 优先并在注释标注）；il-spec §3.6 同步。
5. 🟢 **META.commits 用会话计数**：改用真实链长 `lenLog(store)+1`。

**下一步（M6 续）**
- ✅ 悬空引用检测：`见/参见/参考 R3/A2/D1/?1` 指向不存在条目 → error（`refRe` + 各段 id 索引）。
- ✅ OPEN 收敛提示：OPEN 的 default 措辞已被某条 CONTRACT 决定（default 是 CONTRACT 正文子串）→ suggestion，提示移除。
- ✅ **遵守率实证（v2 定稿依据）**：真实 deepseek-chat 跑 5 轮多样化建档（加规则/改阈值/删规则/闲聊/改错误格式），ComplianceStats 结果——**语法校验拒绝 0 次、一致性 Lint 0 发现**；闲聊轮正确识别为无关消息无 commit；commit 自动摘要准确。本次样本遵守率 100%，未发现需要 v2 修订语言本身的违例模式。v2 定稿方向：维持 v1.0 语法结构，主要收尾已落地的确定性 lint + LLM 语义复查 + META 归 Agent 等语义澄清，暂不需要文法级改动。
## M5 — 真实 LLM 接入（已完成核心验证，见下）

**已完成**
- `provider/deepseek.go`：DeepSeek（OpenAI 兼容）Provider；`Mode=write` 时强制 JSON 双通道输出，`Mode=repro` 时输出纯产物；返回前对 intent_update 做 il 解析+校验+规范化。
- 密钥经 `.env`（已 gitignore）承载，经 `cmd/il/dotenv.go` 加载，不入库。
- 真实闭环验证（deepseek-chat，两个 key 充当 A/B 两模型）：
  1. **双通道写档案**：3 轮对话（建加减 → 加乘 → 加除/除零处理），模型只读【当前档案+需求】，增量重写档案正确：编号递增 R1→R6、ACCEPT 同步、DECISIONS 记录否决、OPEN 更新 default、META.commits 递增。全程无需历史消息 → 验证"档案即记忆"。
  2. **失忆复现**：删会话后用 key B 仅凭最终档案重建出可运行的 Python 计算器；ACCEPT 黑盒用例全数通过（含负号解析、除零提示、非法输入、多运算符拒绝、向下取整遵守 OPEN.default）。
  3. **artifact 层 GUI 复现**：key A 建 tkinter 计算器档案（FIDELITY structure，SNIPPET 布局）→ key B 重建 3733 字节产物，SNIPPET 按钮矩阵逐字一致、无幻觉除法、左到右求值。
- 证据归档：`testdata/calculator_llm_verified.il`（真实 LLM 档案）、`testdata/artifact_verified.py`（复现产物）；`il` 增加回归测试强制真实档案必须通过校验。
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

## M7 — 多档案引用（已完成）

- ✅ 语言语法：<ref: name#id@ver> 成为 v2.0 语法元素（il-spec §2.6）；限 CONTRACT/ACCEPT/ANCHORS。
- ✅ il.ScanRefs/ScanRefsAll + lint 规则（引用出现在禁止段 → error）。
- ✅ internal/refs：Resolve（递归物化 + 循环检测 + 版本校验 + 来源标注），单测覆盖简单/嵌套/循环/版本错/缺失。
- ✅ SpecPrompt 引用说明；SpecVersion 升 v2.0；VS Code grammar 高亮 <ref>。
- ✅ archive 多档案：OpenArchive(dir,name) 独立 commit 链；Store.ArchiveDir/ListArchives；兼容默认 main 存根。
- ✅ CLI：chat/lint/repro/log/show/rollback 支持 --name 档案；repro/lint 从仓库读；repro 喂 LLM 前 Resolve 展开引用。
- ✅ 真实验证：建 common（共享错误规则）→ 建 app 引用 <ref: common#R1@1.0> → repro app 展开引用 → 产物实现 common 规则，行为验收 3/3 通过。
- ✅ accept 支持 --repo/--name（从仓库读并展开引用再生成验收脚本）。
- 待做：跨仓库依赖（远期）。

**工具纵深收尾**
- ✅ verify 升级对齐 v2：支持 --repo/--name（仓库多档案），跑与 chat 同一套 L1/L2 分层逻辑并预览完整档案。
- ✅ compare 废弃：文本相似度不能衡量意图一致性，命令入口保留但提示改用 accept/lint（03-design/05-commands 同步）。
- ✅ stats 补齐：lint 计数分 error/suggestion 汇总。
- ✅ lint --llm SNIPPET 检查方向校准：明确"SNIPPET 代码与 CONTRACT 冲突→error"；真实验证能抓 print(stdout) vs R1(stderr) 冲突，并在 calculator fixture 上发现 SNIPPET 只有 apply 而 ACCEPT 测 add 的真实缺陷。

## M8 — 分层一致性（记录优先，已完成）

> 目标：处理"生成端输出与档案/回复不一致"的问题，且**不干扰 LLM 生成**。

**原则**：IL 是记录介质，不是控制手段。系统只产出"合法且自洽的记录"，不提示、不否决、不扣留、不阻断。

**分层模型**

| 层 | 检查什么 | 谁做 | 触发 |
|---|---|---|---|
| L0 生成 | 不检查、不干扰 | 生成器（自由产出草稿） | 每轮 |
| L1 确定性 | 结构校验 + 机器 diff + lint error | 机器（纯计算） | 每轮 |
| L2 语义 | L1 修不了的非法/矛盾 → 整理成合法自洽 | 记录员（同模型不同角色，`ModeNormalize`） | 仅 L1 报警 |
| L3 行为 | 档案 ↔ 产物 | ACCEPT | 按需 |
| L4 人 | 最终裁决 | 人 | 随时 |

**已完成**
- ✅ **机器 diff 覆盖全档案**：`il.EntryBodies` 统一变更单元（`R/A/D/?` + 头字段 `INTENT/KIND/FIDELITY/TARGET` + `ANCHORS` + `SNIPPET:<label>`）；`CompareEntries` 与 `archive.Summarize` 共用，记录与摘要口径一致。
- ✅ **L2 记录员**：`NormalizePrompt` + `provider.ModeNormalize`；`cmd/il/normalize.go` 的 `l1Problems`/`recordArchive`，仅 L1 报警时调用，不改事实、不拒绝；救不回则按草稿记录。
- ✅ chat/verify/demo 改为分层记录流程；每轮打印机器 diff 权威摘要。
- ✅ reply 只作记录（"模型的话"），权威意图只有档案。
- ✅ **移除**：变更闸门、`declared_changes`、校验拒绝、reply 扣留、`CheckReplyClaims`/`LintReplyPrompt` 提示、`undeclaredChanges`。
- ✅ 单测：头字段/ANCHORS/SNIPPET 的增删改可被 `CompareEntries` 检出。

**设计沿革（实测驱动）**
- ⏪ 曾用"变更闸门 + `declared_changes`"做声明-校验式阻断；实测（8 场景）单次通过仅 3/8，模型对穷举声明遵守度不足、拒绝过多。
- ⏪ 曾把字段顺序改为 declare-first，实测顺序 8/8 遵守但拒绝仍多，已回退。
- ⏪ 曾用 `CheckReplyClaims` / `lint --llm --reply` 做 reply↔档案提示；因 reply 降级为纯记录而移除。

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
