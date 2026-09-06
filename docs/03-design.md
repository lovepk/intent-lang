# 设计文档 (Design) — v0.1

## 1. 总体架构

项目由四个核心部分构成：

```
┌─────────────────────────────────────────────────────────────┐
│                       对话时 (Session 存在)                   │
│                                                             │
│  用户 ──消息──► Agent ──► [规范 + 最新档案B] ──► LLM Provider │
│                      │                ▲                      │
│                      │                │  (正常回答 + 档案更新)  │
│                      ▼                │                      │
│                正常回答返回用户   ┌────┴─────┐                │
│                    + 更新档案B ◄──┤ Agent内核 │                │
│                        │          └──────────┘                │
│                        ▼                                      │
│                ┌───────────────┐                              │
│                │  档案仓库 Archive│  (B 的 commit 历史)         │
│                └───────────────┘                              │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                    失忆后 (Session 已删除)                    │
│                                                             │
│   [IL规范 + 档案B@vN] ──► 复现器 Repro ──► LLM Provider      │
│                                    │                         │
│                                    ▼                         │
│                             产物 C'                           │
│                                    │                         │
│                                    ▼                         │
│                         相似度/验收 报告                       │
└─────────────────────────────────────────────────────────────┘
```

## 2. 模块划分

| 模块 | 职责 | 位置建议 |
|------|------|----------|
| `il` | 意图语言：档案 B 的解析、序列化、校验、规范化（canonical form，供 diff）。 | `internal/il/` |
| `provider` | LLM 抽象：统一请求/响应接口；`mock` 与将来 `openai`/`anthropic` 等实现。 | `internal/provider/` |
| `agent` | 双通道对话内核：装配请求上下文、接收响应、触发档案更新、写 commit。 | `internal/agent/` |
| `archive` | 档案仓库：B 的持久化、版本历史、commit、回滚、加载"最新 B"。 | `internal/archive/` |
| `repro` | 复现器：给定【规范+B】调 Provider 生成 C'；产出物落盘。 | `internal/repro/` |
| `similarity` | 对比 C 与 C'：文本行 diff、结构归一化对比、相似度分数；可运行验收用例。 | `internal/similarity/` |
| `cli` | 命令行入口：驱动以上模块做交互式对话 / 批处理复现。 | `cmd/intent-lang/` |
| `docs` | 文档：本文与目标/场景/规范/计划。 | `docs/` |

## 3. 核心概念与数据流

### 3.1 双通道对话协议

每次用户消息处理都走同一流水线：

1. **装配上下文**：Agent 读取 `IL 规范`（静态，随版本发布）与 `最新档案 B`（来自 Archive）。
2. **请求 Provider**：一次请求。响应被拆为两个通道：
   - 通道 A：`reply`——给用户看的正常回答。
   - 通道 B：`intentUpdate`——由意图语言描述的对档案的修改（见 3.2）。
3. **更新档案**：Agent 校验 `intentUpdate` 合法性，合并到当前 B，生成新版本。
4. **提交 commit**：Archive 写入一条 commit：`{id, prev, message, userMsg, diff, newB, ts}`。
5. **返回**：正常回答给用户；档案变更可回显为"已更新 B v0.1 → v0.2"。

关键设计决策 **DEC-1**：更新档案的方式——**LLM 输出"新的完整档案 B'"，由 Agent 计算 B→B' 的 diff**（而非让 LLM 直接输出 patch）。

理由：
- 档案是自包含规格，完整重写保证"失忆后仅凭 B 可复现"这一硬约束不被 patch 的上下文依赖破坏；
- diff 由确定性代码计算，比让 LLM 生成 patch 稳定得多；
- commit 历史仍然完整（diff 是 Agent 算出来的，一样可审计、可回滚）。

> 备选（将来可支持，作为 Provider 能力开关）：让 LLM 输出结构化 patch。但默认路径是"全量重写 + 程序化 diff"。

### 3.2 响应格式

Provider 的统一响应结构（对 mock 与真实实现一致）：

```json
{
  "reply": "给用户看的文字…",
  "intentUpdate": "<完整的新档案 B 文本>"
}
```

意图语言文本本身见 `il-spec.md`。Agent 对 `intentUpdate` 的期望是"合法 IL 文档"，通过 `il` 模块校验后才能落盘；校验失败时走降级策略（见 6.1）。

### 3.3 复现数据流（Reproduce）

1. 输入：`IL 规范` + `档案 B@vN`（从 Archive 读取或指定文件）。
2. Repro 装配单轮请求：角色=系统(规范) + 用户(档案，要求"严格按此档案重建产物 C'"）。
3. Provider 返回产物文本，落盘为 `C'`。
4. similarity 对比 `C`（对话阶段留存/或由 B 复现基线）与 `C'`，输出报告。

注：mock 阶段 C 与 C' 都由确定性 mock 生成，diff 用于验证"管线本身不引入漂移"；真实 LLM 阶段，C 需要来自原始对话时产物快照，复现报告对比快照与 C'。

### 3.4 档案版本化

Archive 保存线性 commit 链（第一阶段不需要分支）：

```
commit id: c-1   B@v0.1   "init: add & subtract"      ←用户: 做加减计算器
commit id: c-2   B@v0.2   "feat: multiply"            ←用户: 加乘法
commit id: c-3   B@v1.0   "feat: gui tkinter grid"    ←用户: 改GUI
```

支持操作：`log`、`show <id>`、`rollback <id>`、`export <id> > b.intent`。

## 4. Provider 抽象

```go
type Provider interface {
    // Complete 执行一次对话补全；装配内容由 Agent 负责。
    Complete(ctx context.Context, req Request) (Response, error)
}

type Request struct {
    System  string // 系统提示：IL 规范 + 角色约定
    Archive string // 当前最新档案 B（对话模式=最新版；复现模式=目标版）
    User    string // 当前用户消息（对话模式）或复现指令
}

type Response struct {
    Reply        string // 通道 A
    IntentUpdate string // 通道 B：完整的新档案文本（全量重写），可为空
}
```

### mock 实现

第一阶段 `mock` Provider 满足以下性质（使离线可复现且行为可解释）：

- 内置一个**确定性的计算器领域转换器**：从用户消息识别意图（如 `加/乘/界面` 等），把变更映射为对档案各段的修改（追加 `R`、补 `ACCEPT`、更新 `FIDELITY`/`TARGET`、记 `DECISIONS`）。
- **全量重写**：每次响应输出完整的新档案文本（满足 il-spec §4 规则 1）；Agent 负责 diff 与 commit。
- 自校验：输出前解析并 `Validate()`，产出非法档案即报错返回。
- 确定性：纯规则、无随机；相同输入序列 → 相同输出（有测试保证）。
- 通过 `NewMock(name)` 可创建多个不同名实例（模拟 LLM-A / LLM-B），为 M3 跨模型复现验证做准备。

## 5. 意图语言 (IL) 概要

语言本体权威定义见 `il-spec.md`（v1.0）。设计要点摘录：

1. **四条根本原则**：① 消歧优先（只允许"已消歧的事实"）；② 否决与决策是资产（`NO` 否定 + `DECISIONS` 日志）；③ FIDELITY 声明锁定层级（`behavior`/`structure`/`artifact`）；④ 人类可读优先（任何一行不得要求读者"学过编程"）。
2. **八个段落**：头四段 `INTENT/KIND/FIDELITY/TARGET` + `CONTRACT`(R) / `ANCHORS` / `SNIPPET`(黄金代码) / `ACCEPT`(A) / `DECISIONS`(D) / `OPEN`(?，必带 default) / `META`。
3. **稳定 ID 优先**：R/A/D 用稳定编号；新增=追加，修订=改正文保编号，作废=记入 META.deprecated；编号永不复用。
4. **强模板、弱语法**：段落与编号是唯一硬结构，条目一律自然语言句子；语义靠消歧判定保证，不靠文法。所有传统语法要素以自然句形态呈现（见 il-spec 2.5 对照表）。
5. **自包含 + 全量重写**：一次用户消息 = 输出完整新档案（DEC-1），保证"失忆后仅凭 B 可复现"。
6. **规范与档案分离**：IL 语法规范是静态文档（注入用），档案 B 是具体实例。

## 6. 边界与异常处理

### 6.1 Provider 返回非法意图语言

- 校验失败时：不落盘。降级策略 = 把校验错误与 B 原样拼进下一条系统提示，请 LLM 修正；若连续失败超过阈值（如 3 次），本次只返回 reply，不更新档案，并提示用户。
- mock 阶段可注入错误场景用于测试该路径。

### 6.2 产物 C 快照管理

- 对话阶段，每次 commit 可附带 `artifact`（如 C 的快照文本），便于回滚复现对比。
- 计算器场景默认启用 `snapshot`；纯对话场景可关闭以省空间。

### 6.3 相似度阈值与报告

- 报告含：行级 diff 摘要、归一化相似度（0~1）、验收用例逐条通过/失败。
- 达标阈值作为配置项，默认 0.95（`docs/04-plan.md` 中校准）。

## 7. 目录规划

```
intent-lang/
├── go.mod
├── README.md
├── docs/
│   ├── 01-goals.md
│   ├── 02-scenarios.md
│   ├── 03-design.md
│   ├── il-spec.md
│   └── 04-plan.md
├── cmd/intent-lang/main.go
├── internal/
│   ├── il/            # 解析/校验/规范化/生成
│   ├── provider/      # 接口 + mock
│   ├── agent/         # 对话内核
│   ├── archive/       # 档案仓库 + commit
│   ├── repro/         # 复现器
│   ├── similarity/    # 对比报告
│   └── …
└── testdata/          # 计算器 demo 的规范与档案样例
```

## 8. 开放问题

**已拍板**（写入 il-spec v1.0）：

- 语义解释器 = LLM（始终）；IL 不追求可编译文法。
- 人类可读性列为第四原则：非程序员无需编程知识即可读懂（il-spec 2.5 对照表约束了传统语法要素的呈现形态）。
- OQ-3（功能点粒度）：已定——条目为自然语言句子，粒度由"消歧判定"把关，`OPEN` 收纳未消歧项，不再二选一。

**仍开放（进入 M1 编码时定）**：

- **OQ-1**：档案 B 首个版本是否需要独立创建？（当前默认：LLM 见到第一条需求直接产出 B@v1，无空档案阶段。）
- **OQ-2**：历史消息 `History` 何时裁剪？（当 B 足够表达意图时，可把历史压缩为仅【规范+B+当前消息】，对应场景 4。首版先全量保留。）
