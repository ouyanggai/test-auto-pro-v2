#!/usr/bin/env bash

# F-015 写端点白名单检查：本切片仍是纯只读，白名单为空。
# 沿用 F-013 与 F-014 的形式，只扫描 internal/adapter/target——那里是唯一能真正发出目标请求的地方。
# 另外单独确认本切片新增的代码没有引用任何写端点，也没有把 batchCode 带进请求。

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
  if grep -RInF "${endpoint}" internal/adapter/target >/dev/null 2>&1; then
    printf '%s\n' "[F-015] 目标适配层出现了写端点：${endpoint}" >&2
    exit 1
  fi
  for source in "${f015_sources[@]}"; do
    if [ -e "${source}" ] && grep -RInF "${endpoint}" "${source}" >/dev/null 2>&1; then
      printf '%s\n' "[F-015] 本切片代码引用了写端点：${source} -> ${endpoint}" >&2
      exit 1
    fi
  done
done

if grep -RInE 'RequestClass:[[:space:]]*"write"' internal cmd >/dev/null 2>&1; then
  printf '%s\n' '[F-015] 出现了写请求分类，但当前白名单为空' >&2
  exit 1
fi

for source in "${f015_sources[@]}"; do
  if [ -e "${source}" ] && grep -RInF 'batchCode' "${source}" >/dev/null 2>&1; then
    printf '%s\n' "[F-015] 本切片代码出现了 batchCode，违反语义清单第 2.2 节禁令：${source}" >&2
    exit 1
  fi
done

printf '%s\n' 'F-015 目标写端点白名单检查通过（白名单为空，本切片未引用任何写端点）'
