package contracts_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode"
)

var f009ChangedGoFiles = []string{
	"cmd/server/main.go",
	"internal/api/execution_paths.go",
	"internal/api/health.go",
	"internal/api/path_configuration.go",
	"internal/api/path_preparation.go",
	"internal/formdata/generator.go",
	"internal/model/execution_path.go",
	"internal/model/path_config.go",
	"internal/model/path_preparation.go",
	"internal/repository/execution_path.go",
	"internal/repository/path_config.go",
	"internal/repository/path_preparation.go",
	"internal/repository/mysql/execution_path_repository.go",
	"internal/repository/mysql/path_config_repository.go",
	"internal/repository/mysql/path_preparation_repository.go",
	"internal/service/path_config_cycle_copy.go",
	"internal/service/path_config_workspace.go",
	"internal/service/path_form_solver.go",
	"internal/service/path_preparation.go",
	"test/contracts/f008_path_configuration_stub_test.go",
	"test/contracts/f009_path_preparation_api_test.go",
	"test/contracts/f009_go_comments_test.go",
	"test/integration/f009_path_preparation_mysql_test.go",
	"test/unit/backend/execution_path_service_test.go",
	"test/unit/backend/f008_path_config_test.go",
	"test/unit/backend/f009_form_solver_test.go",
	"test/unit/backend/f009_path_preparation_service_test.go",
	"test/unit/backend/form_data_generator_test.go",
}

// TestF009ChangedNamedFunctionsHaveChineseComments 验证本切片修改的具名函数和方法都有紧邻声明的中文职责注释。
func TestF009ChangedNamedFunctionsHaveChineseComments(t *testing.T) {
	root := f009ProjectRoot(t)
	for _, relative := range f009ChangedGoFiles {
		path := filepath.Join(root, relative)
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("解析 %s 失败：%v", relative, err)
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if function.Doc == nil || !containsHan(function.Doc.Text()) {
				t.Errorf("%s 的具名函数 %s 缺少声明正上方中文注释", relative, function.Name.Name)
			}
		}
	}
}

// f009ProjectRoot 从当前测试文件位置解析项目根目录，不依赖执行命令的工作目录。
func f009ProjectRoot(t *testing.T) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位 F-009 契约测试文件")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("无法定位项目根目录：%v", err)
	}
	return root
}

// containsHan 判断注释是否至少包含一个中文汉字。
func containsHan(value string) bool {
	return strings.IndexFunc(value, func(character rune) bool {
		return unicode.Is(unicode.Han, character)
	}) >= 0
}
