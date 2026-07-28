#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
files=(
  cmd/server/main.go
  internal/adapter/target/client.go
  internal/analyzer/execution_path.go
  internal/api/execution_paths.go
  internal/api/flow_graph.go
  internal/api/health.go
  internal/api/plans.go
  internal/repository/mysql/execution_path_repository.go
  internal/repository/mysql/plan_repository.go
  internal/service/execution_path.go
  internal/service/flow_graph.go
  internal/service/target_read.go
  test/contracts/execution_path_api_test.go
  test/contracts/flow_graph_api_test.go
  test/integration/execution_path_mysql_integration_test.go
  test/integration/target_read_integration_test.go
  test/unit/backend/execution_path_analyzer_test.go
  test/unit/backend/execution_path_service_test.go
  test/unit/backend/flow_graph_service_test.go
)

failed=0
for relative_file in "${files[@]}"; do
  file="${project_root}/${relative_file}"
  previous=""
  line_number=0
  while IFS= read -r line || [[ -n "${line}" ]]; do
    line_number=$((line_number + 1))
    if [[ "${line}" == func\ * ]]; then
      name="$(sed -E 's/^func (\([^)]*\) )?([A-Za-z0-9_]+).*/\2/' <<<"${line}")"
      if [[ "${previous}" != "// "* || ! "${previous}" =~ [一-龥] ]]; then
        echo "${relative_file}:${line_number} 缺少紧邻函数声明的中文职责注释" >&2
        failed=1
      elif [[ "${name}" =~ ^[A-Z] && "${previous}" != "// ${name}"* ]]; then
        echo "${relative_file}:${line_number} 导出函数注释未以 ${name} 开头" >&2
        failed=1
      fi
    fi
    previous="${line}"
  done <"${file}"
done

if [[ "${failed}" -ne 0 ]]; then
  exit 1
fi

echo 'F-005 Go 中文函数注释结构检查通过'
