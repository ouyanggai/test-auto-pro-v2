#!/usr/bin/env bash
# F-017 写端点白名单契约：本切片不扩张写能力，实际写请求仍是 F-016 两个端点的子集。
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${project_root}"
grep -qF 'WriteEndpointSubmit = "/web/flowInstanceApi/submit"' internal/adapter/target/write.go
grep -qF 'WriteEndpointAudit = "/flowInstanceApi/audit"' internal/adapter/target/write.go
# F-019 起白名单按动作目录扩张为 11 个端点（docs/features/F-019-*.md）；
# CallWrite 的调用点只允许在 write.go（发起/同意）与 write_actions.go（统一动作出口）。
call_sites=$(grep -rn 'c.CallWrite(ctx' internal/adapter/target/ | grep -v '_test')
[ -n "${call_sites}" ]
echo "${call_sites}" | grep -vqE 'write(_actions)?\.go:' && {
  printf '%s\n' '[F-017] CallWrite 调用只允许在 write.go 与 write_actions.go 里' >&2
  exit 1
}
if [ "$(grep -c 'CallWrite(ctx' internal/adapter/target/write.go)" != "2" ]; then
  printf '%s\n' '[F-017] 写出口调用必须仍然只有发起与同意两处' >&2
  exit 1
fi
printf '%s\n' '[F-017] 写端点白名单契约通过（未扩张）'
