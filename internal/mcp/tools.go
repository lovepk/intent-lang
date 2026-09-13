package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"intent-lang/internal/agent"
	"intent-lang/internal/archive"
	"intent-lang/internal/il"
)

// tool is one MCP tool: a name, a description, a JSON schema and a handler.
type tool struct {
	name        string
	description string
	schema      map[string]any
	handler     func(args map[string]any) (string, error)
}

func (s *Server) tools() []tool {
	ts := []tool{
		{
			name:        "il_parse",
			description: "解析一份 IL 档案文本，返回头部字段、各段行数与规范化后的档案。",
			schema:      objSchema(map[string]any{"text": strProp("IL 档案文本")}, "text"),
			handler:     toolParse,
		},
		{
			name:        "il_lint",
			description: "对档案做确定性检查（结构校验 + 一致性 lint），返回问题清单。不调用模型。",
			schema:      objSchema(map[string]any{"text": strProp("IL 档案文本")}, "text"),
			handler:     toolLint,
		},
		{
			name:        "il_diff",
			description: "比较两版档案，返回新增/删除/修改的变更单元（含头字段、ANCHORS、SNIPPET）。",
			schema: objSchema(map[string]any{
				"before": strProp("旧档案文本"),
				"after":  strProp("新档案文本"),
			}, "before", "after"),
			handler: toolDiff,
		},
		{
			name:        "il_resolve",
			description: "把档案中的 <ref: name#entry@ver> 引用展开成自包含文本（复现前必须做）。",
			schema: objSchema(map[string]any{
				"text": strProp("含引用的档案文本"),
				"repo": strProp("档案仓库目录（解析引用用，默认服务启动时的 --repo）"),
				"name": strProp("档案名（默认 main）"),
			}, "text"),
			handler: s.toolResolve,
		},
		{
			name:        "il_read",
			description: "读取仓库中某档案的最新提交文本。",
			schema: objSchema(map[string]any{
				"repo": strProp("档案仓库目录"),
				"name": strProp("档案名（默认 main）"),
			}),
			handler: s.toolRead,
		},
		{
			name:        "il_commit",
			description: "把一次档案更新落库为一条 commit（确定性：diff 摘要 + META 由本工具计算）。after 为完整新档案文本。",
			schema: objSchema(map[string]any{
				"after":    strProp("更新后的完整档案文本（必填）"),
				"before":   strProp("更新前的档案文本（省略则取仓库最新提交）"),
				"user_msg": strProp("触发本次更新的用户原话"),
				"reply":    strProp("本次给用户的回答"),
				"repo":     strProp("档案仓库目录"),
				"name":     strProp("档案名（默认 main）"),
			}, "after"),
			handler: s.toolCommit,
		},
		{
			name:        "il_log",
			description: "列出仓库中某档案的提交历史。",
			schema: objSchema(map[string]any{
				"repo": strProp("档案仓库目录"),
				"name": strProp("档案名（默认 main）"),
			}),
			handler: s.toolLog,
		},
	}
	if s.model != nil {
		ts = append(ts, s.orchestratorTools()...)
	}
	return ts
}

// orchestratorTools are the optional, model-driven tools. They are only
// advertised when a model is configured (see Server.WithModel).
func (s *Server) orchestratorTools() []tool {
	return []tool{
		{
			name:        "il_record",
			description: "（可选，会调用服务端配置的模型）双通道一轮：给定当前档案与用户需求，返回 reply + 更新后的完整档案与变更单元。不落库；需要持久化请再调 il_commit。",
			schema: objSchema(map[string]any{
				"archive":  strProp("当前档案文本（可为空，表示从零开始）"),
				"user_msg": strProp("用户本次需求（必填）"),
			}, "user_msg"),
			handler: s.toolRecord,
		},
		{
			name:        "il_verify",
			description: "（可选，会调用服务端配置的模型）单轮预览：对仓库当前档案（或给定 archive）应用一条需求，返回 reply + 变更，不落库。等价于 CLI verify。",
			schema: objSchema(map[string]any{
				"archive":  strProp("档案文本；省略则用 repo/name 读仓库最新"),
				"repo":     strProp("仓库目录"),
				"name":     strProp("档案名（默认 main）"),
				"user_msg": strProp("用户本次需求（必填）"),
			}, "user_msg"),
			handler: s.toolVerify,
		},
		{
			name:        "il_lint_semantic",
			description: "（可选，会调用服务端配置的模型）语义复查：找档案内部需要理解力的矛盾（ACCEPT 与被否功能冲突、SNIPPET 与 CONTRACT 冲突等）。",
			schema: objSchema(map[string]any{
				"archive": strProp("档案文本；省略则用 repo/name 读仓库最新"),
				"repo":    strProp("仓库目录"),
				"name":    strProp("档案名（默认 main）"),
			}),
			handler: s.toolLintSemantic,
		},
		{
			name:        "il_reproduce",
			description: "（可选，会调用服务端配置的模型）仅凭档案重建产物。给定 archive 文本，或省略并用 repo/name 从仓库读取（自动展开 <ref>）。",
			schema: objSchema(map[string]any{
				"archive": strProp("自包含档案文本；省略则用 repo/name"),
				"repo":    strProp("仓库目录"),
				"name":    strProp("档案名（默认 main）"),
			}),
			handler: s.toolReproduce,
		},
		{
			name:        "il_accept",
			description: "（可选，会调用服务端配置的模型）把 ACCEPT 用例翻译成测试并对产物运行，返回生成的脚本与验收结果。",
			schema: objSchema(map[string]any{
				"archive":  strProp("档案文本；省略则用 repo/name"),
				"repo":     strProp("仓库目录"),
				"name":     strProp("档案名（默认 main）"),
				"artifact": strProp("产物文件路径（必填）"),
			}, "artifact"),
			handler: s.toolAccept,
		},
	}
}

func (s *Server) toolRecord(args map[string]any) (string, error) {
	userMsg := argString(args, "user_msg")
	if userMsg == "" {
		return "", fmt.Errorf("user_msg is required")
	}
	before := argString(args, "archive")
	if before == "" {
		if o := s.sourceOpts(args); o.Repo != "" || o.Name != "" {
			if text, err := agent.LoadSourceRaw(o); err == nil {
				before = text
			}
		}
	}
	return s.recordTurn(before, userMsg)
}

func (s *Server) toolVerify(args map[string]any) (string, error) {
	userMsg := argString(args, "user_msg")
	if userMsg == "" {
		return "", fmt.Errorf("user_msg is required")
	}
	source := argString(args, "archive")
	if source == "" {
		text, err := agent.LoadSourceRaw(s.sourceOpts(args))
		if err != nil {
			return "", err
		}
		source = text
	}
	return s.recordTurn(source, userMsg)
}

func (s *Server) recordTurn(before, userMsg string) (string, error) {
	turn, err := (&agent.Agent{Model: s.model}).Record(context.Background(), before, userMsg)
	if err != nil {
		return "", err
	}
	return renderTurn(turn), nil
}

func renderTurn(turn agent.Turn) string {
	var b strings.Builder
	b.WriteString("reply:\n" + turn.Reply + "\n")
	if !turn.Changed {
		b.WriteString("\n(档案未变更)")
		return b.String()
	}
	b.WriteString("\n--- archive ---\n" + turn.Archive)
	if changed := turn.Diff.Changed(); len(changed) > 0 {
		b.WriteString("\n\nchanged: " + strings.Join(changed, ", "))
	}
	if turn.Normalized {
		b.WriteString("\n(L1 发现问题，L2 记录员已整理)")
	}
	return b.String()
}

func (s *Server) toolLintSemantic(args map[string]any) (string, error) {
	archiveText := argString(args, "archive")
	if archiveText == "" {
		text, err := agent.LoadSourceRaw(s.sourceOpts(args))
		if err != nil {
			return "", err
		}
		archiveText = text
	}
	findings, err := (&agent.Agent{Model: s.model}).LintLLM(context.Background(), archiveText)
	if err != nil {
		return "", err
	}
	if len(findings) == 0 {
		return "LLM 复查: 未发现问题", nil
	}
	var b strings.Builder
	for _, f := range findings {
		sev := f.Severity
		if sev == "" {
			sev = "suggestion"
		}
		where := f.Section
		if f.ID != "" {
			where += " " + f.ID
		}
		fmt.Fprintf(&b, "[%s] %s: %s\n", sev, where, f.Msg)
		if f.Fix != "" {
			b.WriteString("  建议: " + f.Fix + "\n")
		}
	}
	return b.String(), nil
}

func (s *Server) toolReproduce(args map[string]any) (string, error) {
	archiveText := argString(args, "archive")
	if archiveText == "" {
		resolved, err := agent.LoadResolvedSource(s.sourceOpts(args))
		if err != nil {
			return "", err
		}
		archiveText = resolved
	} else {
		expanded, err := agent.ExpandWithRepo(s.sourceOpts(args), archiveText)
		if err != nil {
			return "", err
		}
		archiveText = expanded
	}
	return (&agent.Agent{Model: s.model}).Reproduce(context.Background(), archiveText)
}

func (s *Server) toolAccept(args map[string]any) (string, error) {
	artifact := argString(args, "artifact")
	if artifact == "" {
		return "", fmt.Errorf("artifact is required")
	}
	archiveText := argString(args, "archive")
	if archiveText == "" {
		resolved, err := agent.LoadResolvedSource(s.sourceOpts(args))
		if err != nil {
			return "", err
		}
		archiveText = resolved
	} else {
		expanded, err := agent.ExpandWithRepo(s.sourceOpts(args), archiveText)
		if err != nil {
			return "", err
		}
		archiveText = expanded
	}
	script, output, err := (&agent.Agent{Model: s.model}).Accept(context.Background(), archiveText, artifact)
	if err != nil {
		return "", err
	}
	return "== 生成的验收脚本 ==\n" + script + "\n== 验收结果 ==\n" + output, nil
}

func (s *Server) toolList() []map[string]any {
	out := make([]map[string]any, 0, len(s.tools()))
	for _, t := range s.tools() {
		out = append(out, map[string]any{
			"name":        t.name,
			"description": t.description,
			"inputSchema": t.schema,
		})
	}
	return out
}

func (s *Server) callTool(params json.RawMessage) (any, *rpcError) {
	var p struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, badParams("invalid tools/call params: %v", err)
	}
	for _, t := range s.tools() {
		if t.name == p.Name {
			return toolResult(t.handler(p.Arguments))
		}
	}
	return nil, &rpcError{Code: codeInvalidParams, Message: "unknown tool: " + p.Name}
}

// sourceOpts resolves repo/name from tool arguments, falling back to the
// server's startup defaults.
func (s *Server) sourceOpts(args map[string]any) agent.SourceOpts {
	repo := argString(args, "repo")
	if repo == "" {
		repo = s.repo
	}
	name := argString(args, "name")
	if name == "" {
		name = s.name
	}
	return agent.SourceOpts{Archive: argString(args, "archive"), Repo: repo, Name: name}
}

func toolParse(args map[string]any) (string, error) {
	text := argString(args, "text")
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("text is required")
	}
	doc, err := il.Parse(text)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "INTENT %s\nKIND %s\nFIDELITY %s\nTARGET %s\n\n",
		doc.Header.Intent, doc.Header.Kind, doc.Header.Fidelity, doc.Header.Target)
	for _, sec := range doc.Sections {
		fmt.Fprintf(&b, "%s: %d 行\n", sec.Name, len(sec.Lines))
	}
	b.WriteString("\n--- canonical ---\n")
	b.WriteString(doc.Canonical())
	return b.String(), nil
}

func toolLint(args map[string]any) (string, error) {
	text := argString(args, "text")
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("text is required")
	}
	doc, err := il.Parse(text)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, e := range doc.Validate() {
		b.WriteString("[error] " + e.Error() + "\n")
	}
	for _, d := range doc.Lint() {
		b.WriteString(d.String() + "\n")
	}
	if b.Len() == 0 {
		return "lint: 通过，未发现一致性问题", nil
	}
	return b.String(), nil
}

func toolDiff(args map[string]any) (string, error) {
	before := argString(args, "before")
	after := argString(args, "after")
	d := il.CompareEntries(before, after)
	var b strings.Builder
	for _, id := range d.Added {
		fmt.Fprintf(&b, "+ %s\n", id)
	}
	for _, id := range d.Removed {
		fmt.Fprintf(&b, "- %s\n", id)
	}
	for _, id := range d.Modified {
		fmt.Fprintf(&b, "~ %s\n", id)
	}
	if b.Len() == 0 {
		return "无变更", nil
	}
	return b.String(), nil
}

func (s *Server) toolResolve(args map[string]any) (string, error) {
	text := argString(args, "text")
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("text is required")
	}
	return agent.ExpandWithRepo(s.sourceOpts(args), text)
}

func (s *Server) toolRead(args map[string]any) (string, error) {
	return agent.LoadSourceRaw(s.sourceOpts(args))
}

func (s *Server) toolCommit(args map[string]any) (string, error) {
	after := argString(args, "after")
	if strings.TrimSpace(after) == "" {
		return "", fmt.Errorf("after is required")
	}
	store, err := agent.OpenStore(s.sourceOpts(args))
	if err != nil {
		return "", err
	}
	before := argString(args, "before")
	if before == "" {
		if latest, _ := store.Latest(); latest != nil {
			before = latest.Archive
		}
	}
	commits := 1
	if log, err := store.Log(); err == nil {
		commits = len(log) + 1
	}
	created, deprecated := agent.ParseMetaCarry(before)
	deprecated = agent.NextDeprecated(deprecated, before, after)
	summary := archive.Summarize(before, after)
	c, err := store.Append(summary, argString(args, "user_msg"), before, after,
		argString(args, "reply"), agent.BuildMetaLines(commits, created, deprecated))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("commit %s: %s", c.ID, c.Message), nil
}

func (s *Server) toolLog(args map[string]any) (string, error) {
	store, err := agent.OpenStore(s.sourceOpts(args))
	if err != nil {
		return "", err
	}
	log, err := store.Log()
	if err != nil {
		return "", err
	}
	if len(log) == 0 {
		return "(empty repo)", nil
	}
	var b strings.Builder
	for i, c := range log {
		marker := "  "
		if i == 0 {
			marker = "* "
		}
		fmt.Fprintf(&b, "%s%s %s | %s | %s\n", marker, c.ID, c.Time.Format("2006-01-02T15:04:05Z07:00"), c.Message, c.UserMsg)
	}
	return b.String(), nil
}

// --- schema + argument helpers ---

func objSchema(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func strProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func argString(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	if v, ok := args[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}
