#!/usr/bin/env bash

# F-016 写端点白名单契约：实际存在的目标写调用必须是两个白名单端点的子集。
# 纲领第 10 节：写能力的扩张过程本身要被测试守住。

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${project_root}"

allowed_submit='/web/flowInstanceApi/submit'
allowed_audit='/flowInstanceApi/audit'

printf '%s\n' '[F-016] 白名单常量恰好为两个端点'
grep -qF "WriteEndpointSubmit = \"${allowed_submit}\"" internal/adapter/target/write.go
grep -qF "WriteEndpointAudit = \"${allowed_audit}\"" internal/adapter/target/write.go

printf '%s\n' '[F-016] CallWrite 只被两个白名单端点使用'
write_callers=$(grep -rn 'CallWrite(' internal/adapter/target/ | grep -v '_test' | grep -v 'func (c \*Client) CallWrite' || true)
[ -n "${write_callers}" ]
while IFS= read -r line; do
  file="${line%%:*}"
  # F-019 的统一动作写出口按动作目录分派端点（已获批扩张），由 test/contracts/f019/action_whitelist.sh 单独守住；
  # 本契约继续保证 F-016 自己的两个直连写路径（write.go）不扩张。
  if [ "${file}" = "internal/adapter/target/write_actions.go" ]; then
    continue
  fi
  grep -qF "WriteEndpointSubmit" "${file}" || grep -qF "WriteEndpointAudit" "${file}" || {
    printf '[F-016] 非白名单文件调用了写出口：%s\n' "${file}" >&2
    exit 1
  }
done <<< "${write_callers}"

printf '%s\n' '[F-016] 写请求分类 write 只在白名单端点的调用链上'
grep -rn 'RequestClassFromContext\|callOfClass' internal/adapter/target/client.go >/dev/null

printf '%s\n' '[F-016] 执行器与适配层不携带 batchCode'
if grep -rn 'batchCode' internal/engine/step/ internal/adapter/target/write.go | grep -v '_test' | grep -v '不携带\|禁令\| Forbidden\|ValidateWritePayload\|禁止'; then
  printf '%s\n' '[F-016] 发现 batchCode 字面量（注释除外）' >&2
  exit 1
fi

printf '%s\n' '[F-016] 引擎除 step 外不得调用写出口'
if grep -rln 'CallWrite' internal/engine/ | grep -v 'internal/engine/step/'; then
  printf '%s\n' '[F-016] 写出口只能在适配层与 step 装配链上' >&2
  exit 1
fi

printf '%s\n' '[F-016] 其余九个写端点未被任何工具代码引用'
for endpoint in reSubmit storageFormData approverAppend rollBackThePreviousLevel retrieveProcess revocation sendUrgeMessage forward follow; do
  if grep -rq "flowInstanceApi/${endpoint}\|${endpoint}" internal/adapter/target/write.go 2>/dev/null; then
    printf '[F-016] 白名单外写端点出现：%s\n' "${endpoint}" >&2
    exit 1
  fi
done

printf '%s\n' '[F-016] 写端点白名单契约通过'
