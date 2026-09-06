# intent-lang

一种给 LLM 使用的意图语言 (Intent Language) 及其 agent 工作流：让"人与 LLM 的对话"沉淀为一份**独立于会话、可移植、可复现的意图档案**。

## 核心思想

```
会话中（人机都记得）
  用户 ──消息──► agent ──► LLM ──► ① 正常回答
                                    ② 意图档案 B 更新（像程序员改代码）
  经过 n 次交流 → 产物 C 完成，档案 B@vN 完成

会话删除（人机双失忆）
  [IL 规范 + 档案 B@vN] ──► 任意 LLM ──► 重建 C' ≈ C（相似度可测）
```

一切依赖如下四层：

- **意图档案 B**：用 IL 编写的自包含规格，是唯一的持久资产。
- **IL 规范**：注入给 LLM 的语言规则（见 `docs/il-spec.md`）。
- **双通道协议**：LLM 每次既回答用户，又像改代码一样产出新一版 B。
- **复现闭环**：删掉会话后，仅凭【规范 + B】即可重建产物，并有相似度报告验收。

## 文档

| 文档 | 内容 |
|------|------|
| `docs/01-goals.md` | 问题、目标、非目标、术语、成功判据 |
| `docs/02-scenarios.md` | 核心场景（计算器 golden path、跨模型、回滚等） |
| `docs/03-design.md` | 架构、数据流、Provider 抽象、边界处理 |
| `docs/il-spec.md` | 意图语言规范 v1.0（给 LLM 读的语言定义） |
| `docs/04-plan.md` | 里程碑 M0–M5、验收标准、风险 |

## 状态

- 当前：**M0** —— 文档与骨架。
- 技术栈：Go。第一阶段使用 mock LLM（离线可复现），真实 LLM 以可替换接口接入（`internal/provider`）。

## 用法

（M2–M4 完成后补齐）预期命令：

```sh
go run ./cmd/intent-lang chat            # 交互式双通道对话
go run ./cmd/intent-lang reproduce -b b@vN   # 失忆复现 + 相似度报告
go run ./cmd/intent-lang log / rollback <id> # 档案版本管理
```

## 路线

M0 文档 → M1 IL+mock → M2 对话内核+档案仓库 → M3 复现+相似度 → M4 端到端演示 → M5 真实 LLM。
