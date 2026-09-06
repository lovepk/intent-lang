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
	if req.Mode == ModeRepro {
		return mockRepro(req)
	}
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

func mockRepro(req Request) (Response, error) {
	doc, err := il.Parse(req.Archive)
	if err != nil {
		return Response{}, fmt.Errorf("mock repro: parse: %w", err)
	}
	ops := mockOpsInArchive(doc)
	hasGUI := false
	for _, op := range ops {
		if op == "gui" {
			hasGUI = true
		}
	}

	body := new(strings.Builder)
	genOpFuncs(body, ops)
	if hasGUI {
		genGUI(body)
	} else {
		genCLI(body, ops)
	}
	return Response{Reply: body.String()}, nil
}

func mockOpsInArchive(doc *il.Doc) []string {
	ops := []string{}
	sec := doc.Section("CONTRACT")
	if sec == nil {
		return ops
	}
	for _, line := range sec.Lines {
		switch {
		case strings.Contains(line, "add("):
			ops = appendUnique(ops, "add")
		case strings.Contains(line, "subtract("):
			ops = appendUnique(ops, "subtract")
		case strings.Contains(line, "multiply("):
			ops = appendUnique(ops, "multiply")
		case strings.Contains(line, "divide("):
			ops = appendUnique(ops, "divide")
		case strings.Contains(line, "tkinter") || strings.Contains(line, "4x4"):
			ops = appendUnique(ops, "gui")
		}
	}
	if len(ops) == 0 {
		return []string{"add", "subtract"}
	}
	return ops
}

func appendUnique(list []string, s string) []string {
	for _, v := range list {
		if v == s {
			return list
		}
	}
	return append(list, s)
}

func genOpFuncs(b *strings.Builder, ops []string) {
	for _, op := range ops {
		switch op {
		case "add":
			fmt.Fprintf(b, "def add(a, b):\n    return a + b\n")
		case "subtract":
			fmt.Fprintf(b, "def subtract(a, b):\n    return a - b\n")
		case "multiply":
			fmt.Fprintf(b, "def multiply(a, b):\n    return a * b\n")
		case "divide":
			fmt.Fprintf(b, "def divide(a, b):\n    if b == 0:\n        raise ZeroDivisionError\n    return a / b\n")
		}
	}
}

func genCLI(b *strings.Builder, ops []string) {
	fmt.Fprintf(b, "def main():\n")
	fmt.Fprintf(b, "    while True:\n")
	fmt.Fprintf(b, "        try:\n")
	fmt.Fprintf(b, "            line = input('> ').strip()\n")
	fmt.Fprintf(b, "        except EOFError:\n")
	fmt.Fprintf(b, "            break\n")
	fmt.Fprintf(b, "        if not line or line.lower() in ('quit', 'exit'):\n")
	fmt.Fprintf(b, "            break\n")
	fmt.Fprintf(b, "        parts = line.split()\n")
	fmt.Fprintf(b, "        if len(parts) != 3:\n")
	fmt.Fprintf(b, "            print('invalid input')\n")
	fmt.Fprintf(b, "            continue\n")
	fmt.Fprintf(b, "        a, op, b = parts\n")
	fmt.Fprintf(b, "        try:\n")
	fmt.Fprintf(b, "            a, b = int(a), int(b)\n")
	fmt.Fprintf(b, "        except ValueError:\n")
	fmt.Fprintf(b, "            print('invalid input')\n")
	fmt.Fprintf(b, "            continue\n")
	fmt.Fprintf(b, "        fn = {\n")
	for _, op := range ops {
		switch op {
		case "add":
			fmt.Fprintf(b, "            '+': add,\n")
		case "subtract":
			fmt.Fprintf(b, "            '-': subtract,\n")
		case "multiply":
			fmt.Fprintf(b, "            '*': multiply,\n")
		case "divide":
			fmt.Fprintf(b, "            '/': divide,\n")
		}
	}
	fmt.Fprintf(b, "        }\n")
	fmt.Fprintf(b, "        if op not in fn:\n")
	fmt.Fprintf(b, "            print('invalid input')\n")
	fmt.Fprintf(b, "            continue\n")
	fmt.Fprintf(b, "        try:\n")
	fmt.Fprintf(b, "            print(fn[op](a, b))\n")
	fmt.Fprintf(b, "        except ZeroDivisionError:\n")
	fmt.Fprintf(b, "            print('division by zero')\n")
	fmt.Fprintf(b, "\n")
	fmt.Fprintf(b, "if __name__ == '__main__':\n")
	fmt.Fprintf(b, "    main()\n")
}

func genGUI(b *strings.Builder) {
	fmt.Fprintf(b, "import tkinter as tk\n")
	fmt.Fprintf(b, "\n")
	fmt.Fprintf(b, "class Calc(tk.Tk):\n")
	fmt.Fprintf(b, "    def __init__(self):\n")
	fmt.Fprintf(b, "        super().__init__()\n")
	fmt.Fprintf(b, "        self.title('calc')\n")
	fmt.Fprintf(b, "        self.display = tk.Entry(self)\n")
	fmt.Fprintf(b, "        self.display.grid(row=0, column=0, columnspan=4)\n")
	fmt.Fprintf(b, "        keys = ['7','8','9','/','4','5','6','*','1','2','3','-','0','.','+','=']\n")
	fmt.Fprintf(b, "        r, c = 1, 0\n")
	fmt.Fprintf(b, "        for k in keys:\n")
	fmt.Fprintf(b, "            tk.Button(self, text=k, command=lambda k=k: self.press(k)).grid(row=r, column=c)\n")
	fmt.Fprintf(b, "            c += 1\n")
	fmt.Fprintf(b, "            if c > 3:\n")
	fmt.Fprintf(b, "                c = 0\n")
	fmt.Fprintf(b, "                r += 1\n")
	fmt.Fprintf(b, "    def press(self, k):\n")
	fmt.Fprintf(b, "        if k == '=':\n")
	fmt.Fprintf(b, "            try:\n")
	fmt.Fprintf(b, "                r = eval(self.display.get())\n")
	fmt.Fprintf(b, "                self.display.delete(0, 'end')\n")
	fmt.Fprintf(b, "                self.display.insert(0, str(r))\n")
	fmt.Fprintf(b, "            except Exception:\n")
	fmt.Fprintf(b, "                self.display.delete(0, 'end')\n")
	fmt.Fprintf(b, "                self.display.insert(0, 'error')\n")
	fmt.Fprintf(b, "        else:\n")
	fmt.Fprintf(b, "            self.display.insert('end', k)\n")
	fmt.Fprintf(b, "\n")
	fmt.Fprintf(b, "if __name__ == '__main__':\n")
	fmt.Fprintf(b, "    Calc().mainloop()\n")
}
