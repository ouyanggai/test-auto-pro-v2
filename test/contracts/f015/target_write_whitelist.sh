#!/usr/bin/env bash

# F-015 写端点白名单检查：本切片仍是纯只读，白名单为空。
# 只扫描本切片新增的代码——适配层全局的写端点纪律自 F-016 起由 test/contracts/f016 与 f019
# 的白名单契约承接（用户批准的写能力扩张），本契约继续保证运行准备代码不引用写端点、不携带 batchCode。

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${project_root}"

write_endpoints=(
  '/web/flowInstanceApi/submit'
  '/web/flowInstanceApi/reSubmit'
  '/web/flowInstanceApi/storageFormData'
  '/web/flowInstanceApi/approverAppend'
  '/flowInstanceApi/audit'
  '/web/flowInstanceApi/rollBackThePreviousLevel'
  '/web/flowInstanceApi/retrieveProcess'
  '/web/flowInstanceApi/revocation'
  '/web/urgeHandleRecord/sendUrgeMessage'
  '/web/flowInstanceApi/transpond'
  '/web/flowInstanceApi/flowTracking'
)

f015_sources=(
  internal/service/run_readiness.go
  internal/service/run_readiness_service.go
  internal/api/run_readiness.go
  web/src/features/run-readiness
)

for endpoint in "${write_endpoints[@]}"; do
  for source in "${f015_sources[@]}"; do
    if [ -e "${source}" ] && grep -RInF "${endpoint}" "${source}" >/dev/null 2>&1; then
      printf '%s\n' "[F-015] 本切片代码引用了写端点：${source} -> ${endpoint}" >&2
      exit 1
    fi
  done
done

for source in "${f015_sources[@]}"; do
  if [ -e "${source}" ] && grep -RInF 'batchCode' "${source}" >/dev/null 2>&1; then
    printf '%s\n' "[F-015] 本切片代码出现了 batchCode，违反语义清单第 2.2 节禁令：${source}" >&2
    exit 1
  fi
done

printf '%s\n' 'F-015 目标写端点白名单检查通过（本切片代码未引用任何写端点）'
