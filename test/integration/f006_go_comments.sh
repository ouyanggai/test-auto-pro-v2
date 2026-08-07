#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
files=(
  cmd/server/main.go
  internal/adapter/target/client.go
  internal/analyzer/path_requirement.go
  internal/api/health.go
  internal/api/path_requirements.go
  internal/service/path_requirement.go
  internal/service/target_read.go
  test/contracts/path_requirement_api_test.go
  test/integration/target_read_integration_test.go
  test/unit/backend/path_requirement_analyzer_test.go
  test/unit/backend/path_requirement_service_test.go
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

echo 'F-006 Go 中文函数注释结构检查通过'
