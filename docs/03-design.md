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

| 模块 | 职责 | 位置 |
|------|------|------|
| `il` | 意图语言：档案 B 的解析、序列化、校验、规范化（canonical form，供 diff）、META 剥离/注入（StripMeta/WithMeta）、SpecPrompt/ReproPrompt。 | `internal/il/` |
| `provider` | LLM 抽象：`Provider` 接口 + `deepseek`（真实）+ `Retry` 装饰器 + `StripFence`。 | `internal/provider/` |
| `archive` | 档案仓库：commit 链持久化（`.intent/` 目录）、自动 commit message 摘要、log/show/rollback。 | `internal/archive/` |
| `cli` | 命令行入口：`chat`/`verify`/`repro`/`compare`/`accept`/`lint`/`demo`/`log`/`show`/`rollback`。 | `cmd/intent-lang/` |
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

Provider 的统一响应结构（对各真实实现一致）：

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

注：真实 LLM 阶段，C 需要来自原始对话时产物快照，复现报告对比快照与 C'。

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
    Mode    Mode   // ModeWrite（双通道 JSON）或 ModeRepro（纯产物输出）
}

type Response struct {
    Reply        string // 通道 A（write）或重建产物（repro）
    IntentUpdate string // 通道 B：完整的新档案文本（全量重写），可为空
}
```

### 真实 Provider：DeepSeek

`provider/deepseek.go` 实现 OpenAI 兼容协议：

- `Mode=write`：请求启用 `response_format: json_object`，模型必须返回 `{"reply","intent_update"}`；返回前对 intent_update 做 il 解析+校验+规范化（非法即报错，由 `Retry` 装饰器带纠正指令重试）。
- `Mode=repro`：不启用 JSON 模式，模型直接输出产物；经 `StripFence` 防御性清洗后返回。
- 凭据经 `.env` 提供 `DEEPSEEK_API_KEY_A/B`（对应 LLM-A/LLM-B），加载于 CLI 层。
- `provider.Retry` 装饰器：对非法 JSON / 非法档案的输出，把纠正指令拼回下一次请求重试（默认 2 次）。

## 5. 意图语言 (IL) 概要

语言本体权威定义见 `il-spec.md`（v1.0）。设计要点摘录：

1. **四条根本原则**：① 消歧优先（只允许"已消歧的事实"）；② 否决与决策是资产（`NO` 否定 + `DECISIONS` 日志）；③ FIDELITY 声明锁定层级（`behavior`/`structure`/`artifact`）；④ 人类可读优先（任何一行不得要求读者"学过编程"）。
2. **八个段落**：头四段 `INTENT/KIND/FIDELITY/TARGET` + `CONTRACT`(R) / `ANCHORS` / `SNIPPET`(黄金代码) / `ACCEPT`(A) / `DECISIONS`(D) / `OPEN`(?，必带 default) / `META`。
3. **稳定 ID 优先**：R/A/D 用稳定编号；新增=追加，修订=改正文保编号，作废=移除条目（`deprecated` 由 Agent 依据 diff 计算写入 META）；编号永不复用。
4. **强模板、弱语法**：段落与编号是唯一硬结构，条目一律自然语言句子；语义靠消歧判定保证，不靠文法。所有传统语法要素以自然句形态呈现（见 il-spec 2.5 对照表）。
5. **自包含 + 全量重写**：一次用户消息 = 输出完整新档案（DEC-1），保证"失忆后仅凭 B 可复现"。
6. **规范与档案分离**：IL 语法规范是静态文档（注入用），档案 B 是具体实例。

## 6. 边界与异常处理

### 6.1 Provider 返回非法意图语言

- 校验失败时：不落盘。降级策略 = 由 `provider.Retry` 把校验错误与纠正指令拼进下一次请求，请 LLM 修正；若连续失败超过阈值（默认 2 次），本次只返回 reply，不更新档案，并提示用户（实现于 `provider/retry.go`）。

### 6.2 产物 C 快照管理

- 对话阶段，每次 commit 可附带 `artifact`（如 C 的快照文本），便于回滚复现对比。
- 计算器场景默认启用 `snapshot`；纯对话场景可关闭以省空间。

### 6.3 双轨验收与报告

复现一致性按两层验收（与期望表述一致）：

- **行为轨（默认，适用 behavior 及以上）**：用档案 ACCEPT 用例对可运行产物做黑盒验收，逐条通过/失败。这是"复现成功"的主判据。
- **形态轨（仅对点名锁定部分）**：对被 SNIPPET/ANCHORS 点名的代码块/布局做文本或结构 diff（LCS 归一化分数）。达标阈值作为配置项，默认 0.95（`docs/04-plan.md` 中校准）。
- 报告含：ACCEPT 逐条结果 + （点名部分）行级 diff 摘要与归一化相似度。

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
├── cmd/intent-lang/   # main / chat / verify / repro / compare / accept / lint / demo / repo(log,show,rollback) / dotenv
├── internal/
│   ├── il/            # 解析/校验/规范化/Meta保护/提示词
│   ├── provider/      # 接口 + deepseek + retry + strip
│   ├── archive/       # 档案仓库 + commit 链 + 摘要
│   ├── repro/         # 复现器
│   ├── similarity/    # 对比报告
│   └── …
└── testdata/          # 计算器档案样例 + 真实 LLM 验证产物
```

## 8. 开放问题

**已拍板**（写入 il-spec v1.0 与实现）：

- 语义解释器 = LLM（始终）；IL 不追求可编译文法。
- 人类可读性列为第四原则：非程序员无需编程知识即可读懂（il-spec 2.5 对照表约束了传统语法要素的呈现形态）。
- OQ-3（功能点粒度）：已定——条目为自然语言句子，粒度由"消歧判定"把关，`OPEN` 收纳未消歧项，不再二选一。
- OQ-1：档案首个版本不预置骨架，LLM 见到第一条需求直接产出档案（实现验证 OK）。
- OQ-2：不做 History——请求恒为【规范+最新 B+当前消息】（场景 4 已验证成立；这也是"档案即记忆"的机制）。
- 空档案幻觉防护（实测发现）：无档案时 LLM 会虚构提交历史；Agent 在 archive 为空时应显式让 LLM"从零建档 v0.1"，此分支由 Agent 控制而非模型自行判断。当前 `chat` 在 archive 为空时传空字符串，已依赖 SpecPrompt"空档案=从零开始"约定，未来接仓库时需显式断言。

**遗留（进入 M4 处理）**：

- ACCEPT 自动行为验收器（对可运行产物逐条跑用例）。
- M4 端到端演示命令 + 双轨阈值校准。
