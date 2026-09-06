# CLI 命令参考（intent-lang）

> 构建/运行：`go run ./cmd/il <命令> [选项]`（或先 `go build -o intent-lang.exe ./cmd/il` 再用 `il <命令>`）。
> 真实调用需 `.env` 提供 `DEEPSEEK_API_KEY_A/B`；`--key A|B` 切换两个凭据充当 LLM-A / LLM-B。

## 一、命令总览（10 个）

| 分组 | 命令 | 作用 | 常用参数 |
|---|---|---|---|
| **建档** | `chat` | 交互式双通道对话：输入需求 → reply + 档案增量更新，每次变更自动写一条 commit | `--key` `--repo` |
| **质量** | `verify` | 单轮无副作用预览：给定档案+需求，看模型会怎么改（不写 commit） | `--key` `--archive` `--msg` |
| | `lint` | 语言体检：确定性结构检查；`--llm` 追加 LLM 语义复查 | `--archive` `--key`(llm) |
| **复现** | `repro` | 失忆复现：仅凭档案重建产物 | `--key` `--archive` `--out` |
| **验收** | `accept` | ACCEPT 行为验收：LLM 把验收用例翻译成测试并运行 | `--key` `--archive` `--artifact` |
| | `compare` | 两产物形态对比（LCS diff + 相似度） | `--ref` `--cand` `--threshold` `--fidelity` |
| **演示** | `demo` | 端到端 golden path：key A 建档 → key B 复现 → 验收（一条命令跑全流程） | `--script` `--repo` |
| **档案管理** | `log` | 查看 commit 历史 | `--repo` |
| | `show` | 查看某条 commit 详情（默认最新） | `--repo` `[id]` |
| | `rollback` | 回滚 HEAD 到某条 commit | `--repo` `[id]` |

**全局约定**：
- `--key A|B`：选哪个凭据（默认 A）。A 通常当"建档端"，B 当"复现端"（跨模型验证）。
- `--repo <dir>`：档案仓库目录（默认 `.il`）。chat/log/show/rollback/demo 用。
- `--retry N`：非法输出自动带纠正指令重试次数（默认 2，chat/repro/accept/lint 生效）。
- 位置参数：`show <id>` / `rollback <id>` / `lint <file.il>` 可把 id/路径直接放命令后。

---

## 二、命令详解

### 1. `chat` — 双通道对话建档（核心入口）

输入需求（一行一条），模型正常回答的同时更新档案；每次档案变更落一条 commit；退出打印规范遵守统计。

```sh
il chat --key A --repo .il
> 做一个 CLI 温度转换工具，摄氏转华氏
> 加上华氏转摄氏
> exit
```

- 对话结束自动写出 `.il/archive.last.il`（最新档案，供 repro/lint 使用）。
- 变更受**闸门**保护：模型改了却没在 `declared_changes` 里声明的条目会被拒绝落库。
- 无关消息（闲聊）不产生 commit。

### 2. `verify` — 单轮无副作用预览

不进仓库、不写 commit，直接看"给定这条需求，模型会怎样重写档案"。用于试需求、验证遵守度。

```sh
il verify --key A --archive calc.il --msg "加上乘法"
# 输出 reply + 模型重写后的完整档案（合法才显示）
```

> 与 `chat` 的区别：`chat` 会落库成 commit，`verify` 只预览不落库。

### 3. `lint` — 语言体检

两层检查：
- **确定性结构 lint**：SNIPPET/FIDELITY 不匹配、META.spec 版本、OPEN 空 default、悬空引用、OPEN 已收敛等（快、零成本）。
- `--llm` **LLM 语义复查**：找需要理解力的矛盾（如 ACCEPT 测被 CONTRACT 否定的功能），领域无关。

```sh
il lint --archive calc.il              # 结构检查
il lint --llm --key A --archive calc.il # + LLM 复查
```

### 4. `repro` — 失忆复现

模拟"会话删除、人机双失忆"，仅凭档案重建产物。默认输出名从档案文件名+TARGET 推断扩展名。

```sh
il repro --key B --archive .il/archive.last.il --out artifact.py
```

### 5. `accept` — ACCEPT 行为验收

LLM 依据档案 ACCEPT 用例生成测试脚本，本机运行，输出 PASS/FAIL/TOTAL。可跑产物时才有效；GUI 等由脚本降级为源码检查或跳过。

```sh
il accept --archive calc.il --artifact artifact.py
# 依赖本机 python（验收脚本以标准库跑产物）；解释器可用 IL_PYTHON 覆盖
```

### 6. `compare` — 产物形态对比

行级 LCS diff + 相似度。默认 `--fidelity structure` 才做阈值判定；`behavior` 层仅提示改用 ACCEPT 行为验收。

```sh
il compare --ref a.py --cand b.py --threshold 0.95 --fidelity structure
```

### 7. `demo` — 端到端一键演示

一条命令跑完整 golden path：key A 从脚本多轮建档 → 删会话 → key B 仅凭档案复现 → ACCEPT 验收 → SNIPPET 点名行检查。

```sh
il demo --script turns.txt --repo .il-demo
# turns.txt：每行一条用户需求
```

### 8. `log` / `show` / `rollback` — commit 历史管理

```sh
il log --repo .il               # 列出所有 commit（* = HEAD）
il show --repo .il              # 最新 commit 详情（档案全文）
il show c-1788xxx --repo .il    # 指定 commit
il rollback c-1788xxx --repo .il  # 回到该版本（HEAD 移到它）
```

每条 commit 含：id / parent / message（机器 diff 自动摘要）/ user_msg（用户原话）/ 完整档案快照 / AI 回复 / 时间。

---

## 三、典型工作流

```sh
# 1. 建档（多轮需求 → commit 链）
il chat --key A --repo .il

# 2. 体检
il lint --archive .il/archive.last.il
il lint --llm --key A --archive .il/archive.last.il

# 3. 失忆复现（换 key B，证明档案可移植）
il repro --key B --archive .il/archive.last.il --out artifact.py

# 4. 行为验收
il accept --archive .il/archive.last.il --artifact artifact.py

# 5. 反悔/查账
il log --repo .il
il rollback c-1788xxx --repo .il
```
