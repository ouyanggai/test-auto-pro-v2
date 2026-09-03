#!/usr/bin/env bash

# F-013 写端点白名单检查：本切片白名单为空，目标适配层不允许出现任何写端点。
# 只扫描 internal/adapter/target：那里是唯一能真正发出目标请求的地方。
# internal/engine/actioncatalog 里的 targetOperation 只是动作目录的说明元数据，
# 描述"该动作在未来执行时会调用哪个接口"，不构成一次请求，因此不在扫描范围内。

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${project_root}"

# 当前白名单为空：一个元素都不允许出现在运行代码里。
write_endpoints=(
  '/web/flowInstanceApi/submit'
  '/web/flowInstanceApi/reSubmit'
  '/web/flowInstanceApi/audit'
  '/web/flowInstanceApi/revocation'
  '/web/flowInstanceApi/abandon'
  '/web/flowInstanceApi/delete'
  '/web/flowInstanceApi/rollBackThePreviousLevel'
  '/web/flowInstanceApi/approverAppend'
  '/web/flowInstanceApi/retrieveProcess'
  '/web/flowInstanceApi/storageFormData'
  '/web/flowInstanceApi/appendSubprocess'
)

for endpoint in "${write_endpoints[@]}"; do
  if grep -RInF "${endpoint}" internal/adapter/target >/dev/null 2>&1; then
    printf '%s\n' "[F-013] 目标适配层出现了写端点：${endpoint}" >&2
    exit 1
  fi
done

# 请求分类只允许 read：写分类出现意味着有人在没有扩充白名单的情况下加了写请求。
if grep -RInE 'RequestClass:[[:space:]]*"write"' internal cmd >/dev/null 2>&1; then
  printf '%s\n' '[F-013] 出现了写请求分类，但当前白名单为空' >&2
  exit 1
fi

printf '%s\n' 'F-013 目标写端点白名单检查通过（白名单为空）'
