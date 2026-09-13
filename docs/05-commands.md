# CLI 命令参考（intent-lang）

> 构建/运行：`go run ./cmd/il <命令> [选项]`（或先 `go build -o intent-lang.exe ./cmd/il` 再用 `il <命令>`）。
> 真实调用需 `.env` 提供 `DEEPSEEK_API_KEY_A/B`；`--key A|B` 切换两个凭据充当 LLM-A / LLM-B。

## 一、命令总览（9 个）

| 分组 | 命令 | 作用 | 常用参数 |
|---|---|---|---|
| **建档** | `chat` | 交互式双通道对话：输入需求 → reply + 档案增量更新，每次变更自动写一条 commit | `--key` `--repo` |
| **质量** | `verify` | 单轮无副作用预览：给定档案+需求，看模型会怎么改（不写 commit） | `--key` `--archive` `--msg` |
| | `lint` | 语言体检：确定性结构检查；`--llm` 追加 LLM 语义复查 | `--archive` `--key`(llm) |
| **复现** | `repro` | 失忆复现：仅凭档案重建产物 | `--key` `--archive` `--out` |
| **验收** | `accept` | ACCEPT 行为验收：LLM 把验收用例翻译成测试并运行 | `--key` `--archive` `--artifact` |
| **演示** | `demo` | 端到端 golden path：key A 建档 → key B 复现 → 验收（一条命令跑全流程） | `--script` `--repo` |
| **档案管理** | `log` | 查看 commit 历史 | `--repo` |
| | `show` | 查看某条 commit 详情（默认最新） | `--repo` `[id]` |
| | `rollback` | 回滚 HEAD 到某条 commit | `--repo` `[id]` |

**全局约定**：
- `--key A|B`：选哪个凭据（默认 A）。A 通常当"建档端"，B 当"复现端"（跨模型验证）。
- `--repo <dir>`：档案仓库目录（默认 `.il`）。chat/log/show/rollback/demo 用。
- `--name <档案名>`：操作哪个档案（v2 多档案）。默认 `main` 存仓库根；命名档案存 `<repo>/<name>/`，各自独立 commit 链，可互相用 `<ref: name#entry@ver>` 引用。
- `--retry N`：非法输出自动带纠正指令重试次数（默认 2，chat/repro/accept/lint 生效）。
- 位置参数：`show <id>` / `rollback <id>` / `lint <file.il>` 可把 id/路径直接放命令后。

**多档案示例**（共享规范 + 引用）：
```sh
il chat --name common          # 建共享规范（如统一错误处理）
il chat --name app             # 建应用，其条目可写 <ref: common#R1@1.0>
il lint --repo .il --name app  # 检查 app 源档案
il repro --repo .il --name app # 从仓库读 app、自动展开 <ref> 成自包含档案再复现
```

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
- **只记录、不干扰**：生成端自由产出；系统一律记录，不拒绝、不重试、不扣留、不提示。
- **分层一致性**：L1 确定性（结构校验 + 机器 diff + lint）每轮跑；仅当 L1 发现非法/矛盾时才触发 L2 记录员（同模型不同角色，`ModeNormalize`）把草稿整理成合法自洽档案；记录员救不回则按草稿记录。
- 无关消息（闲聊）不产生 commit。
- 每轮打印 `--- 档案实际变更（权威，机器 diff）---` 摘要（变更单元含 `R/A/D/?`、头字段、`ANCHORS`、`SNIPPET:<label>`）。

### 2. `verify` — 单轮无副作用预览（v2）

不进仓库、不写 commit，直接看"给定这条需求，模型会怎样重写档案"。用于试需求。v2 起支持从仓库多档案读取。

```sh
il verify --key A --archive calc.il --msg "加上乘法"          # 单文件
il verify --key A --repo .il --name app --msg "加上除法"       # 仓库多档案
# 输出: reply + 机器 diff 摘要 + 分层处理后的档案（未提交）
```

> 与 `chat` 的区别：`chat` 会落库成 commit，`verify` 只预览不落库。verify 跑同一套 L1/L2 分层逻辑。

### 3. `lint` — 语言体检

两层检查：
- **确定性结构 lint（L1）**：SNIPPET/FIDELITY 不匹配、META.spec 版本、OPEN 空 default、悬空引用、OPEN 已收敛、引用出现在禁止段、最小档案等（快、零成本）。
- `--llm` **LLM 语义复查（L2）**：找需要理解力的矛盾（ACCEPT 与被否功能冲突、SNIPPET 锁定代码与 CONTRACT 冲突、引用一致性），领域无关。

```sh
il lint --archive calc.il              # 结构检查
il lint --repo .il --name app          # 检查仓库内某档案
il lint --llm --key A --archive calc.il # + LLM 复查
```

### 4. `repro` — 失忆复现

模拟"会话删除、人机双失忆"，仅凭档案重建产物。从仓库读取时会自动把 `<ref>` 引用展开成自包含档案再复现。默认输出名从档案名+TARGET 推断扩展名。

```sh
il repro --key B --archive .il/archive.last.il --out artifact.py   # 单文件
il repro --key B --repo .il --name app --out app.py               # 仓库多档案（自动展开引用）
```

### 5. `accept` — ACCEPT 行为验收

LLM 依据档案 ACCEPT 用例生成测试脚本，本机运行，输出 PASS/FAIL/TOTAL。可跑产物时才有效；GUI 等由脚本降级为源码检查或跳过。支持单文件或仓库多档案（`--repo`/`--name`，自动展开引用）。

```sh
il accept --archive calc.il --artifact artifact.py
il accept --repo .il --name app --artifact app.py
# 依赖本机 python（验收脚本以标准库跑产物）；解释器可用 IL_PYTHON 覆盖
```

### 6. `compare` — 已废弃

> ⚠️ **已废弃**：文本相似度不能衡量意图一致性（实测同一档案不同模型复现，文本相似度可低至 0.12 但行为全对）。
> 衡量一致性的正确工具是：`accept`（ACCEPT 行为验收）与 `lint`。命令入口仍保留但会提示废弃。

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
