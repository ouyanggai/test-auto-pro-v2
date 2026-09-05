#!/usr/bin/env bash
# F-017 写端点白名单契约：本切片不扩张写能力，实际写请求仍是 F-016 两个端点的子集。
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${project_root}"
grep -qF 'WriteEndpointSubmit = "/web/flowInstanceApi/submit"' internal/adapter/target/write.go
grep -qF 'WriteEndpointAudit = "/flowInstanceApi/audit"' internal/adapter/target/write.go
# F-017 未新增任何写端点或写方法：CallWrite 的定义与两处调用都必须在 write.go。
call_sites=$(grep -rn 'c.CallWrite(ctx' internal/adapter/target/ | grep -v '_test')
[ -n "${call_sites}" ]
echo "${call_sites}" | grep -vqE 'write\.go:' && {
  printf '%s\n' '[F-017] CallWrite 调用只允许在 write.go 里' >&2
  exit 1
}
[ "$(echo "${call_sites}" | wc -l | tr -d ' ')" = "2" ] || {
  printf '%s\n' '[F-017] 写出口调用必须仍然只有发起与同意两处' >&2
  exit 1
}
if [ "$(grep -c 'CallWrite(ctx' internal/adapter/target/write.go)" != "2" ]; then
  printf '%s\n' '[F-017] 写出口调用必须仍然只有发起与同意两处' >&2
  exit 1
fi
printf '%s\n' '[F-017] 写端点白名单契约通过（未扩张）'
