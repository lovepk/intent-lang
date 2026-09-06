package provider

import (
	"context"
	"fmt"
	"strings"

	"intent-lang/internal/il"
)

type Mock struct {
	name string
}

func NewMock(name string) *Mock {
	if name == "" {
		name = "mock-calculus"
	}
	return &Mock{name: name}
}

func (m *Mock) Name() string { return m.name }

func (m *Mock) Complete(ctx context.Context, req Request) (Response, error) {
	doc, err := il.Parse(req.Archive)
	if err != nil && strings.TrimSpace(req.Archive) != "" {
		return Response{}, fmt.Errorf("mock: parse archive: %w", err)
	}
	if doc == nil {
		doc = &il.Doc{HeaderRaw: map[string]string{}}
	}

	created := !isCalc(doc) && len(doc.Sections) == 0
	ops := detectOps(req.User)
	ui := wantsUI(req.User)

	var added []string
	if created {
		if len(ops) == 0 {
			ops = []string{"add", "subtract"}
		}
		buildCalcBase(doc, ops)
		added = ops
	} else if isCalc(doc) {
		for _, op := range ops {
			if !hasOp(doc, op) {
				addOp(doc, op)
				added = append(added, op)
			}
		}
	}

	uiApplied := false
	if isCalc(doc) && ui && !hasUI(doc) {
		addUI(doc)
		uiApplied = true
	}

	if len(added) == 0 && !uiApplied {
		return Response{
			Reply: "收到（mock 判定本次与产物无关，档案未变更）。",
		}, nil
	}

	canon := doc.Canonical()
	if _, err := il.Parse(canon); err != nil {
		return Response{}, fmt.Errorf("mock: produced unparseable archive: %w", err)
	}
	if errs := doc.Validate(); len(errs) != 0 {
		return Response{}, fmt.Errorf("mock: produced archive fails validation: %v", errs)
	}

	var bits []string
	if created {
		bits = append(bits, fmt.Sprintf("已创建计算器档案，支持：%s", joinOps(added)))
	} else if len(added) > 0 {
		bits = append(bits, fmt.Sprintf("已添加：%s", joinOps(added)))
	}
	if uiApplied {
		bits = append(bits, "已将界面切换为 tkinter GUI")
	}
	return Response{Reply: strings.Join(bits, "；") + "。", IntentUpdate: canon}, nil
}

func buildCalcBase(doc *il.Doc, ops []string) {
	doc.HeaderRaw["INTENT"] = "calculator@0.0.1"
	doc.HeaderRaw["KIND"] = "program"
	doc.HeaderRaw["FIDELITY"] = "behavior"
	doc.HeaderRaw["TARGET"] = "python@3.12, single-file, cli"
	for _, op := range ops {
		addOp(doc, op)
	}
	addAccept(doc, "input follows left-to-right without precedence", "compute \"1+2=\" as 3")
	doc.AddLine("ANCHORS", "style: pure functions for calc; main reads stdin line, prints result")
	doc.AddLine("OPEN", "?1: error display in cli     default: print \"Error\" to stderr, exit code 1")
}

func addOp(doc *il.Doc, op string) {
	rID := doc.NextID("CONTRACT")
	var rLine string
	switch op {
	case "add":
		rLine = "add(a,b) -> a+b"
	case "subtract":
		rLine = "subtract(a,b) -> a-b"
	case "multiply":
		rLine = "multiply(a,b) -> a*b"
	case "divide":
		rLine = "divide(a,b) -> a/b; guard b!=0 raise ZeroDivisionError"
	default:
		return
	}
	doc.AddLine("CONTRACT", fmt.Sprintf("%s: %s", rID, rLine))
	aID := doc.NextID("ACCEPT")
	var aLine string
	switch op {
	case "add":
		aLine = "add(1,2) == 3"
	case "subtract":
		aLine = "subtract(5,2) == 3"
	case "multiply":
		aLine = "multiply(3,4) == 12"
	case "divide":
		aLine = "divide(1,0) raises ZeroDivisionError"
	}
	if aLine != "" {
		doc.AddLine("ACCEPT", fmt.Sprintf("%s: %s", aID, aLine))
	}
}

func addAccept(doc *il.Doc, line string, _ string) {
	aID := doc.NextID("ACCEPT")
	doc.AddLine("ACCEPT", fmt.Sprintf("%s: %s", aID, line))
}

func addUI(doc *il.Doc) {
	doc.HeaderRaw["FIDELITY"] = "artifact"
	doc.HeaderRaw["TARGET"] = "python@3.12, single-file, tkinter-grid"
	rID := doc.NextID("CONTRACT")
	doc.AddLine("CONTRACT", fmt.Sprintf("%s: ui is a tkinter window: top display line + 4x4 button grid (0-9 . + - * / = C)", rID))
	aID := doc.NextID("ACCEPT")
	doc.AddLine("ACCEPT", fmt.Sprintf("%s: app launches and shows the grid with all listed buttons", aID))
	sID := doc.NextID("DECISIONS")
	doc.AddLine("DECISIONS", fmt.Sprintf("%s: tkinter chosen as gui     reject: web/cli     due: user asked for desktop gui", sID))
}

func detectOps(msg string) []string {
	norm := msg
	for _, verb := range []string{"加上", "添加", "新增", "增加", "加个", "加一个"} {
		norm = strings.ReplaceAll(norm, verb, " ")
	}
	var out []string
	for _, c := range []struct {
		kw []string
		op string
	}{
		{[]string{"加", "add", "加法"}, "add"},
		{[]string{"减", "subtract", "减法"}, "subtract"},
		{[]string{"乘", "multiply", "乘法"}, "multiply"},
		{[]string{"除", "divide", "除法"}, "divide"},
	} {
		for _, k := range c.kw {
			if strings.Contains(norm, k) {
				out = append(out, c.op)
				break
			}
		}
	}
	return out
}

func wantsUI(msg string) bool {
	l := strings.ToLower(msg)
	return strings.Contains(l, "gui") || strings.Contains(l, "tkinter") ||
		strings.Contains(l, "界面") || strings.Contains(l, "窗口")
}

func isCalc(doc *il.Doc) bool {
	return strings.Contains(doc.HeaderRaw["INTENT"], "calculator") ||
		strings.Contains(doc.HeaderRaw["INTENT"], "计算器")
}

func hasOp(doc *il.Doc, op string) bool {
	sec := doc.Section("CONTRACT")
	if sec == nil {
		return false
	}
	for _, line := range sec.Lines {
		if strings.Contains(line, op+"(") {
			return true
		}
	}
	return false
}

func hasUI(doc *il.Doc) bool {
	sec := doc.Section("CONTRACT")
	if sec == nil {
		return false
	}
	for _, line := range sec.Lines {
		if strings.Contains(line, "tkinter") {
			return true
		}
	}
	return false
}

func joinOps(ops []string) string {
	return strings.Join(ops, "、")
}
