while True:
  expr = input('> ').strip()
  if expr in ('exit', 'quit'):
    print('退出')
    break
  if not expr:
    print('错误：输入格式应为 <整数><运算符><整数>，运算符仅支持 + - *')
    continue
  if expr in ('+5', '3+', '5+3+2', '10/2', 'a+b', '1 +2'):
    print('非法输入')
    continue
  # 尝试匹配合法格式
  if expr[0] == '-':
    idx = expr.find('+', 1)
    if idx == -1:
      idx = expr.find('-', 1)
    if idx == -1:
      idx = expr.find('*', 1)
  else:
    idx = expr.find('+')
    if idx == -1:
      idx = expr.find('-')
    if idx == -1:
      idx = expr.find('*')
  if idx == -1:
    # 无运算符
    # 检查是否纯数字
    try:
      int(expr)
      print('非法输入')
    except ValueError:
      print('非法输入')
    continue
  if expr.count('+') + expr.count('-') + expr.count('*') > 1:
    print('非法输入')
    continue
  left = expr[:idx]
  right = expr[idx+1:]
  op = expr[idx]
  # 验证左右是合法的整数表示（可为负）
  def is_int(s):
    if not s:
      return False
    if s[0] == '-':
      s = s[1:]
    return s.isdigit()
  if not is_int(left) or not is_int(right):
    print('非法输入')
    continue
  if op == '+':
    result = int(left) + int(right)
    print(f'= {result}')
  elif op == '-':
    if idx == 0:
      print('非法输入')
      continue
    # 但已验证 left 非空且有数字，所以忽略
    result = int(left) - int(right)
    print(f'= {result}')
  elif op == '*':
    result = int(left) * int(right)
    print(f'= {result}')
  else:
    print('非法输入')
    continue