#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
files=(
  cmd/server/main.go
  internal/adapter/target/client.go
  internal/adapter/target/types.go
  internal/analyzer/path_config.go
  internal/api/health.go
  internal/api/form_runtime_maintenance.go
  internal/api/path_configuration.go
  internal/model/path_config.go
  internal/repository/path_config.go
  internal/repository/mysql/path_config_repository.go
  internal/service/path_config.go
  internal/service/target_read.go
  internal/formruntimemaintenance/model.go
  internal/formruntimemaintenance/source.go
  internal/formruntimemaintenance/sync.go
  internal/formruntimemaintenance/memory_store.go
  internal/formruntimemaintenance/mysql_store.go
  internal/formruntimemaintenance/log_store.go
  internal/formruntimemaintenance/service.go
  internal/formruntimemaintenance/pipeline.go
  internal/formruntimemaintenance/pnpm_operator.go
  test/contracts/form_runtime_maintenance_api_test.go
  test/contracts/path_configuration_api_test.go
  test/integration/path_config_mysql_integration_test.go
  test/integration/target_read_integration_test.go
  test/integration/form_runtime_maintenance_mysql_integration_test.go
  test/unit/backend/form_runtime_maintenance_test.go
  test/unit/backend/path_config_analyzer_test.go
  test/unit/backend/path_config_service_test.go
)

failed=0
for relative_file in "${files[@]}"; do
  file="${project_root}/${relative_file}"
  lines=()
  while IFS= read -r line || [[ -n "${line}" ]]; do
    lines+=("${line}")
  done <"${file}"
  for index in "${!lines[@]}"; do
    line="${lines[$index]}"
    if [[ "${line}" == func* ]]; then
      name="$(sed -E 's/^func (\([^)]*\) )?([A-Za-z0-9_]+).*/\2/' <<<"${line}")"
      comment=""
      cursor=$index
      while [[ $cursor -gt 0 ]] && [[ "${lines[$((cursor - 1))]}" == "// "* ]]; do
        cursor=$((cursor - 1))
      done
      if [[ $cursor -lt $index ]]; then
        comment="${lines[$cursor]}"
      fi
      if [[ -z "${comment}" || ! "${comment}" =~ [一-龥] ]]; then
        echo "${relative_file}:$((index + 1)) 缺少紧邻函数声明的中文职责注释" >&2
        failed=1
      elif [[ "${name}" =~ ^[A-Z] && "${comment}" != "// ${name}"* ]]; then
        echo "${relative_file}:$((index + 1)) 导出函数注释未以 ${name} 开头" >&2
        failed=1
      fi
    fi
  done
done

if [[ "${failed}" -ne 0 ]]; then
  exit 1
fi

echo 'F-007 Go 中文函数注释结构检查通过'
