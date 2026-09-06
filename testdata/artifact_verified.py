import sys


def main() -> None:
    while True:
        try:
            expr = input("请输入算式: ")
        except EOFError:
            break

        if expr.lower() in ('quit', 'exit'):
            break

        # 检查表达式是否包含非法字符（只允许数字、运算符和负号作为操作数前缀）
        # 允许的操作数可以带负号：-5+3、4--2 等
        if not expr or not all(c.isdigit() or c in '+-*/' for c in expr):
            print("非法输入，请重新输入")
            continue

        # 寻找运算符（+、-、*、/），但需要考虑负号的可能位置
        # 简单方案：从左到右扫描，找到第一个不在开头且为 +、-、*、/ 的字符作为运算符
        operator_index = -1
        operator = None

        for i, ch in enumerate(expr):
            if i == 0 and ch == '-':
                continue  # 第一个字符的 - 可能为负号
            if ch in '+-*/':
                operator_index = i
                operator = ch
                break

        # 如果没有找到运算符，或运算符后没有内容，则非法
        if operator is None or operator_index >= len(expr) - 1:
            print("非法输入，请重新输入")
            continue

        left_part = expr[:operator_index]
        right_part = expr[operator_index + 1:]
        
        # 负号可能出现在左操作数中，如 -5+3（此时 - 是负号，+ 是运算符）
        # 但如果表达式以 - 开头，且没有其他运算符，则非法
        # 去掉负号后如果左操作数为空，则非法
        if not left_part or not right_part:
            print("非法输入，请重新输入")
            continue

        # 检查左右部分是否只有数字（可带负号前缀）
        # 将 left_part 和 right_part 尝试转换为整数
        try:
            if not left_part.lstrip('-').isdigit() or not right_part.lstrip('-').isdigit():
                raise ValueError
            a = int(left_part)
            b = int(right_part)
        except ValueError:
            print("非法输入，请重新输入")
            continue

        # 执行运算
        if operator == '+':
            result = a + b
        elif operator == '-':
            result = a - b
        elif operator == '*':
            result = a * b
        elif operator == '/':
            if b == 0:
                print("除数不能为0，请重新输入")
                continue
            # 向下取整，例如 5/2 = 2，-5/2 = -3
            result = a // b
        else:
            # 理论上不会到这里
            print("非法输入，请重新输入")
            continue

        print(result)


if __name__ == "__main__":
    main()