package history_replay_test

import (
	"strings"
	"testing"

	planmysql "test-auto-pro-v2/internal/repository/mysql"
)

// TestSplitMigrationStatementsSeparatesRebuildMigration 验证迁移文件被逐条拆分，
// 使未开启 multiStatements 的驱动可以按顺序执行整个重建脚本。
func TestSplitMigrationStatementsSeparatesRebuildMigration(t *testing.T) {
	content := `-- 重建 F-012 数据模型
SET FOREIGN_KEY_CHECKS = 0;
DELETE FROM test_form_runtime_sync_jobs;
DROP TABLE IF EXISTS test_path_preparation_items;

CREATE TABLE test_execution_path_configs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  note VARCHAR(64) NOT NULL DEFAULT '需要;确认',
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
SET FOREIGN_KEY_CHECKS = 1;
`
	statements := planmysql.SplitMigrationStatements(content)
	if len(statements) != 5 {
		t.Fatalf("语句拆分数量错误：%d %#v", len(statements), statements)
	}
	if statements[0] != "SET FOREIGN_KEY_CHECKS = 0" {
		t.Fatalf("首条语句应为会话级设置：%q", statements[0])
	}
	if statements[4] != "SET FOREIGN_KEY_CHECKS = 1" {
		t.Fatalf("末条语句应恢复外键检查：%q", statements[4])
	}
	if !strings.Contains(statements[3], "'需要;确认'") {
		t.Fatalf("字符串常量内的分号不得拆分语句：%q", statements[3])
	}
	for ordinal, statement := range statements {
		if strings.HasPrefix(statement, "--") || strings.Contains(statement, "重建 F-012") {
			t.Fatalf("第 %d 条语句仍包含注释：%q", ordinal+1, statement)
		}
		if strings.HasSuffix(statement, ";") {
			t.Fatalf("第 %d 条语句不应保留分号：%q", ordinal+1, statement)
		}
	}
}

// TestSplitMigrationStatementsKeepsQuotedAndCommentedSemicolons 覆盖注释、引号与反引号内的分号。
func TestSplitMigrationStatementsKeepsQuotedAndCommentedSemicolons(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []string
	}{
		{name: "行注释含分号", content: "SELECT 1; -- 忽略; 这里\nSELECT 2;", want: []string{"SELECT 1", "SELECT 2"}},
		{name: "井号注释", content: "SELECT 1; # 忽略; 这里\nSELECT 2;", want: []string{"SELECT 1", "SELECT 2"}},
		{name: "块注释", content: "SELECT /* 忽略; */ 1;", want: []string{"SELECT  1"}},
		{name: "反斜杠转义引号", content: `INSERT INTO t VALUES ('a\';b');`, want: []string{`INSERT INTO t VALUES ('a\';b')`}},
		{name: "重复引号转义", content: `INSERT INTO t VALUES ('a'';b');`, want: []string{`INSERT INTO t VALUES ('a'';b')`}},
		{name: "反引号标识符", content: "SELECT `a;b` FROM t;", want: []string{"SELECT `a;b` FROM t"}},
		{name: "双引号常量", content: `SELECT "a;b";`, want: []string{`SELECT "a;b"`}},
		{name: "缺少末尾分号", content: "SELECT 1", want: []string{"SELECT 1"}},
		{name: "空语句被丢弃", content: ";;\n;\n", want: []string{}},
		{name: "减号非注释", content: "SELECT 1--2;", want: []string{"SELECT 1--2"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			statements := planmysql.SplitMigrationStatements(testCase.content)
			if len(statements) != len(testCase.want) {
				t.Fatalf("拆分数量错误：%d %#v", len(statements), statements)
			}
			for index, want := range testCase.want {
				if statements[index] != want {
					t.Fatalf("第 %d 条语句错误：%q 期望 %q", index+1, statements[index], want)
				}
			}
		})
	}
}
