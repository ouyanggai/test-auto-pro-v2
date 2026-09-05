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

# 适配层全局的“零写端点”断言已被用户批准的写能力扩张取代：
# 写端点必须恰好落在 write.go（F-016 两个直连端点）与 write_actions.go（F-019 动作目录分派）内，
# 由 test/contracts/f016 与 f019 的白名单契约逐端点守住；本契约继续锁定 batchCode 禁令。
for endpoint in "${write_endpoints[@]}"; do
  leaks=$(grep -RIlF "${endpoint}" internal/adapter/target | grep -v 'write.go' | grep -v 'write_actions.go' || true)
  if [ -n "${leaks}" ]; then
    printf '%s\n' "[F-014] 写端点泄漏到适配层白名单文件之外：${endpoint} -> ${leaks}" >&2
    exit 1
  fi
done

# batchCode 只允许作为禁令常量出现在判定包里，任何请求载荷构造处都不得出现它。
if grep -RInF 'batchCode' internal/adapter/target internal/engine/actioncatalog cmd >/dev/null 2>&1; then
  printf '%s\n' '[F-014] 请求构造范围内出现了 batchCode，违反语义清单第 2.2 节禁令' >&2
  exit 1
fi

if ! grep -qF 'ForbiddenWriteField = "batchCode"' internal/engine/verdict/catalog.go; then
  printf '%s\n' '[F-014] 判定包缺少 batchCode 禁令常量' >&2
  exit 1
fi

printf '%s\n' 'F-014 契约检查通过（写端点仅存在于两份白名单文件，batchCode 禁令在位）'
