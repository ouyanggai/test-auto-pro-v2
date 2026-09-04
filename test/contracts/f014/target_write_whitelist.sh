#!/usr/bin/env bash

# F-014 写端点白名单检查：本切片白名单仍为空，目标适配层不允许出现任何写端点。
# 与 F-013 同形式，但覆盖动作目录登记的全部 11 个写端点，并额外锁定 batchCode 禁令：
# batchCode 不是幂等键，而是目标平台的批次补偿开关，带上它会让一次失败触发额外的删除写入。
# 判定包 internal/engine/verdict 里出现这些端点字面量是允许的：那是前置拒绝清单的键，不是请求。

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

for endpoint in "${write_endpoints[@]}"; do
  if grep -RInF "${endpoint}" internal/adapter/target >/dev/null 2>&1; then
    printf '%s\n' "[F-014] 目标适配层出现了写端点：${endpoint}" >&2
    exit 1
  fi
done

if grep -RInE 'RequestClass:[[:space:]]*"write"' internal cmd >/dev/null 2>&1; then
  printf '%s\n' '[F-014] 出现了写请求分类，但当前白名单为空' >&2
  exit 1
fi

# batchCode 只允许作为禁令常量出现在判定包里，任何请求载荷构造处都不得出现它。
if grep -RInF 'batchCode' internal/adapter/target internal/engine/actioncatalog cmd >/dev/null 2>&1; then
  printf '%s\n' '[F-014] 请求构造范围内出现了 batchCode，违反语义清单第 2.2 节禁令' >&2
  exit 1
fi

if ! grep -qF 'ForbiddenWriteField = "batchCode"' internal/engine/verdict/catalog.go; then
  printf '%s\n' '[F-014] 判定包缺少 batchCode 禁令常量' >&2
  exit 1
fi

printf '%s\n' 'F-014 目标写端点白名单检查通过（白名单为空，batchCode 禁令在位）'
