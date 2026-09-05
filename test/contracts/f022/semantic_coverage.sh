#!/usr/bin/env bash

# F-022 语义覆盖契约：清单条目数、证据块数量与写端点合并口径。
# 语义清单是唯一载体：漂移检测（f014）按本文件的口径扩展，不另造第二套。

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${project_root}"

fail() {
  printf '%s\n' "[F-022] $1" >&2
  exit 1
}

doc='docs/TARGET_SEMANTICS.md'
[ -f "${doc}" ] || fail "缺少语义清单：${doc}"

# 19 条语义条目：清单以「## N. 名称」编号，第 1 至 19 条必须齐备。
for no in $(seq 1 19); do
  grep -qE "^## ${no}\." "${doc}" || fail "语义清单缺少第 ${no} 条"
done

# 证据块不少于 34 个：```evidence 围栏块是漂移检测的解析单位。
evidence_count=$(awk '/^```evidence/{count++} END{print count+0}' "${doc}")
if [ "${evidence_count}" -lt 34 ]; then
  fail "证据块不足：期望 >=34，实际 ${evidence_count}"
fi

# 写端点合并口径：11 个端点常量必须齐备（超出即先补语义再扩端点）。
endpoint_count=$(grep -h -c 'WriteEndpoint' internal/adapter/target/write.go internal/adapter/target/write_actions.go | awk '{sum+=$1} END {print sum}')
if [ "${endpoint_count}" -lt 11 ]; then
  fail "写端点常量数量异常：期望 >=11，实际 ${endpoint_count}"
fi

# 漂移检测脚本必须存在且可执行（T04 扩展它而不是另造）。
[ -x 'test/contracts/f014/semantics_evidence_drift.sh' ] || fail '漂移检测脚本缺失或不可执行'

printf '%s\n' 'F-022 语义覆盖契约检查通过'
