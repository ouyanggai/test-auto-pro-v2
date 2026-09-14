#!/usr/bin/env bash

# F-035 关键符号漂移检测：参考前端 FlowDialog 与 examineOpinion 的关键构造符号、
# Java FlowSubmitServiceImpl 入口，以及语义清单第 2.2 节 F-035 改写标记必须仍然存在。
# 参考仓库随 make refs-sync 变化；符号消失即判漂移，必须重新勘定协议矩阵，不允许静默通过。

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"
cd "${project_root}"

check() {
  local file="$1" symbol="$2" label="$3"
  if [ ! -f "${file}" ]; then
    printf '%s\n' "[F-035] 参考文件缺失：${file}（${label}）" >&2
    exit 1
  fi
  if ! grep -q "${symbol}" "${file}"; then
    printf '%s\n' "[F-035] 关键符号漂移：${file} 中找不到 ${symbol}（${label}）" >&2
    exit 1
  fi
}

# 目标前端 submit/reSubmit 的构造逻辑锚点（协议矩阵证据）。
check '参考代码/rsh-flow-components/src/views/GroupApproveManage/Submitted/components/FlowDialog.vue' \
  'enterpriseHandleSubmit' 'FormMaking submit 构造入口'
check '参考代码/rsh-flow-components/src/views/GroupApproveManage/Submitted/components/FlowDialog.vue' \
  'param.batchCode = this.batchCode' 'submit 顶层批次号'
check '参考代码/rsh-flow-components/src/views/GroupApproveManage/Submitted/components/FlowDialog.vue' \
  "param.data.flowProxyId = this.flowId" '无表单走 flowProxyId'
check '参考代码/rsh-flow-components/src/views/GroupApproveManage/Submitted/components/FlowDialog.vue' \
  'formProxyId' '有表单走 formProxyId'
check '参考代码/rsh-flow-components/src/views/GroupApproveManage/components/examineOpinion.vue' \
  'handleSubmitCheck' '审批/重提构造入口'
check '参考代码/rsh-flow-components/src/utils/axios.js' \
  'params.projectId' 'axios 拦截器顶层 projectId 注入'
check '参考代码/rsh-flow-components/src/utils/axios.js' \
  'customerCode' 'axios 拦截器 data.customerCode 注入'

# 目标 Java 提交入口锚点。
check '参考代码/java-serve/rsh-cloud-workflow-center/src/main/java/com/rsh/cloud/workflow/center/service/impl/FlowSubmitServiceImpl.java' \
  'class FlowSubmitServiceImpl' '目标提交服务入口'

# 语义清单必须保留 F-035 改写标记与逐接口矩阵：旧的“一律禁止”只允许出现在被改写的引用语境里，
# 即该句必须同时带有“已被 F-035 改写”字样；否则视为旧禁令回潮。
if grep '工具的写请求一律不得携带' docs/TARGET_SEMANTICS.md | grep -vq '已被 F-035 改写'; then
  printf '%s\n' '[F-035] 语义清单出现旧 batchCode 一律禁令，F-035 改写被回退' >&2
  exit 1
fi
if ! grep -q 'batchCode` 按逐接口协议矩阵携带' docs/TARGET_SEMANTICS.md; then
  printf '%s\n' '[F-035] 语义清单缺少 F-035 逐接口矩阵改写标记' >&2
  exit 1
fi

# 判定包不得再导出字段禁令。
if grep -q 'ForbiddenWriteField' internal/engine/verdict/*.go; then
  printf '%s\n' '[F-035] verdict 仍存在无条件字段拦截 ForbiddenWriteField' >&2
  exit 1
fi

printf '%s\n' '[F-035] 关键符号漂移检测通过'
