package mysql

import "strings"

// SplitMigrationStatements 按 MySQL 词法把单个迁移文件拆成可逐条执行的语句。
// 驱动未开启 multiStatements，必须由调用方按顺序在同一连接上执行拆分结果，
// 否则 SET FOREIGN_KEY_CHECKS 之类的会话级设置会作用到错误的连接。
func SplitMigrationStatements(content string) []string {
	statements := make([]string, 0, 32)
	var current strings.Builder
	runes := []rune(content)
	for index := 0; index < len(runes); index++ {
		character := runes[index]
		switch {
		case isLineCommentStart(runes, index):
			index = skipLineComment(runes, index)
		case character == '/' && index+1 < len(runes) && runes[index+1] == '*':
			index = skipBlockComment(runes, index)
		case character == '\'' || character == '"' || character == '`':
			consumed, quoted := readQuoted(runes, index)
			current.WriteString(quoted)
			index = consumed
		case character == ';':
			appendStatement(&statements, current.String())
			current.Reset()
		default:
			current.WriteRune(character)
		}
	}
	appendStatement(&statements, current.String())
	return statements
}

// isLineCommentStart 只把 MySQL 认可的行注释起点视为注释：`-- `、行尾 `--` 与 `#`。
func isLineCommentStart(runes []rune, index int) bool {
	if runes[index] == '#' {
		return true
	}
	if runes[index] != '-' || index+1 >= len(runes) || runes[index+1] != '-' {
		return false
	}
	if index+2 >= len(runes) {
		return true
	}
	next := runes[index+2]
	return next == ' ' || next == '\t' || next == '\n' || next == '\r'
}

// skipLineComment 返回行注释结束位置，保留换行符以免相邻标识符被粘连。
func skipLineComment(runes []rune, index int) int {
	for index < len(runes) && runes[index] != '\n' {
		index++
	}
	return index - 1
}

// skipBlockComment 跳过 /* */ 注释；未闭合时按到文件末尾处理。
func skipBlockComment(runes []rune, index int) int {
	for cursor := index + 2; cursor < len(runes); cursor++ {
		if runes[cursor] == '*' && cursor+1 < len(runes) && runes[cursor+1] == '/' {
			return cursor + 1
		}
	}
	return len(runes)
}

// readQuoted 原样读取引号或反引号包裹的内容，支持反斜杠转义与重复引号转义。
func readQuoted(runes []rune, index int) (int, string) {
	quote := runes[index]
	var quoted strings.Builder
	quoted.WriteRune(quote)
	for cursor := index + 1; cursor < len(runes); cursor++ {
		character := runes[cursor]
		if character == '\\' && quote != '`' && cursor+1 < len(runes) {
			quoted.WriteRune(character)
			quoted.WriteRune(runes[cursor+1])
			cursor++
			continue
		}
		if character == quote {
			if cursor+1 < len(runes) && runes[cursor+1] == quote {
				quoted.WriteRune(character)
				quoted.WriteRune(character)
				cursor++
				continue
			}
			quoted.WriteRune(character)
			return cursor, quoted.String()
		}
		quoted.WriteRune(character)
	}
	return len(runes) - 1, quoted.String()
}

// appendStatement 丢弃只剩空白的片段，避免向服务器发送空语句。
func appendStatement(statements *[]string, statement string) {
	if trimmed := strings.TrimSpace(statement); trimmed != "" {
		*statements = append(*statements, trimmed)
	}
}
