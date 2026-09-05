#!/usr/bin/env bash
# F-019 动作白名单契约：写能力扩张到动作目录声明的 11 个端点，且必须逐端点可追溯。
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${project_root}"

printf '%s\n' '[F-019] 11 个端点常量全部声明'
for const_name in WriteEndpointSubmit WriteEndpointAudit WriteEndpointReSubmit WriteEndpointStorageForm WriteEndpointApproverAppend WriteEndpointRollBack WriteEndpointRetrieve WriteEndpointRevocation WriteEndpointUrge WriteEndpointTranspond WriteEndpointFlowTracking; do
  grep -qE "${const_name}[[:space:]]*=" internal/adapter/target/write_actions.go internal/adapter/target/write.go || {
    printf '[F-019] 缺少端点常量：%s\n' "${const_name}" >&2
    exit 1
  }
done
# 值逐个核对（端点路径精确匹配）
for endpoint in '/web/flowInstanceApi/submit' '/flowInstanceApi/audit' '/web/flowInstanceApi/reSubmit' '/web/flowInstanceApi/storageFormData' '/web/flowInstanceApi/approverAppend' '/web/flowInstanceApi/rollBackThePreviousLevel' '/web/flowInstanceApi/retrieveProcess' '/web/flowInstanceApi/revocation' '/web/urgeHandleRecord/sendUrgeMessage' '/web/flowInstanceApi/transpond' '/web/flowInstanceApi/flowTracking'; do
  grep -qF "\"${endpoint}\"" internal/adapter/target/write_actions.go internal/adapter/target/write.go || {
    printf '[F-019] 缺少端点路径：%s\n' "${endpoint}" >&2
    exit 1
  }
done

printf '%s\n' '[F-019] 每个动作的载荷构造与端点绑定在 BuildActionBody'
grep -qF 'func BuildActionBody' internal/adapter/target/write_actions.go
for action in resubmit storage_form_data transfer add_sign rollback_previous retrieve withdraw urge forward follow unfollow; do
  grep -qE "case .*${action}" internal/adapter/target/write_actions.go || {
    printf '[F-019] 动作 %s 缺少载荷构造分支\n' "${action}" >&2
    exit 1
  }
done

printf '%s\n' '[F-019] 执行器分派不含 batchCode，未登记动作仍拒绝'
grep -qF 'UnverifiedActionError' internal/engine/step/gate.go
if grep -n 'batchCode' internal/adapter/target/write_actions.go internal/engine/step/gate.go | grep -v '禁止\|禁令\|不做\|不携带\|不混淆'; then
  printf '%s\n' '[F-019] 发现 batchCode 字面量（注释除外）' >&2
  exit 1
fi

printf '%s\n' '[F-019] 动作白名单契约通过'
