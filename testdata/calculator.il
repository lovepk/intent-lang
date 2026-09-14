# file: calculator@1.0.0
INTENT calculator@1.0.0
KIND program
FIDELITY artifact
TARGET python@3.12, single-file, tkinter-grid

CONTRACT
  R1: add(a,b) -> a+b
  R2: subtract(a,b) -> a-b
  R3: multiply(a,b) -> a*b
  R4: divide(a,b) -> a/b; guard b!=0 raise ZeroDivisionError
  R5: ui is a tkinter window, top display line + 4x4 button grid
      buttons: digits 0-9, dot, operators + - * /, =, C(clear)
  R6: evaluation follows input order; NO operator precedence
  R7: expression grammar is flat binary ops, "=" terminates and evaluates
  R8: pressing C clears display and pending state

ANCHORS
  in: "2+3x4="   out: "20"
  in: "1/0="     out: shows "Error", app does not crash
  style: calc logic as pure functions in top section; ui class separate
  style: snake_case names; digits laid out 3 columns of 1-9 plus bottom row

SNIPPET eval_core
  def apply(op, a, b):
      if op == "+": return a + b
      if op == "-": return a - b
      if op == "*": return a * b
      if op == "/":
          if b == 0: raise ZeroDivisionError
          return a / b

ACCEPT
  A1: add(1,2) == 3
  A2: subtract(5,2) == 3
  A3: multiply(3,4) == 12
  A4: divide(1,0) raises ZeroDivisionError
  A5: input "2+3x4=" evaluates to "20"
  A6: app launches, window contains a grid with the listed buttons
  A7: divide-by-zero via UI shows "Error" and recovers on next input

DECISIONS
  D1: 无运算符优先级            reject: 先乘除后加减      due: 用户"按输入顺序算"
  D2: 单文件部署                reject: 多模块             due: 便于直接运行演示
  D3: "x" 作为乘号, "=" 立即求值  reject: 运算符两边等待      due: 用户示例 "2+3x4="

OPEN
  ?1: 除零错误在 GUI 上的呈现方式      default: 结果显示 "Error"; A4 语义不受影响
  ?2: 键盘输入支持                    default: 仅鼠标点击按钮, 不做键盘绑定

META
  spec: v1.0
  created: 2026-09-06T00:00:00Z
  commits: 3
  deprecated: []
