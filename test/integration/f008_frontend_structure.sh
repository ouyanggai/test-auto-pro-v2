#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
panel="${project_root}/web/src/features/path-configuration/NodeConfigurationPanel.vue"
view="${project_root}/web/src/views/PlanPathConfigurationView.vue"
model="${project_root}/internal/model/path_config.go"
workspace="${project_root}/internal/service/path_config_workspace.go"
api="${project_root}/internal/api/path_configuration.go"
plans_api="${project_root}/internal/api/plans.go"
plans_view="${project_root}/web/src/views/PlansView.vue"

grep -Fq '>动作配置</n-button>' "${panel}"
grep -Fq '>循环配置</n-button>' "${panel}"
grep -Fq '重新提交会从发起人开始重新解析条件、并行和人员' "${panel}"
grep -Fq '回退目标由引擎按当前待办的真实上一节点决定' "${panel}"
grep -Fq '动作次数' "${panel}"
grep -Fq 'actionCycles' "${view}"
grep -Fq '纳入本次测试' "${view}"
grep -Fq '已准备 {{ configuration.preparation.preparedNodes }} 个节点' "${view}"
grep -Fq 'PathConfigActionCycle' "${model}"
grep -Fq 'restart_from_initiator' "${workspace}"
grep -Fq 'redo_previous_task' "${workspace}"
grep -Fq '暂存、加签、移交和转发不能加入静态循环' "${workspace}"
grep -Fq '循环只保存服务端从当前路径派生出的事实' "${workspace}"
grep -Fq 'configuration/selection' "${api}"
grep -Fq 'SaveSelection' "${workspace}"
grep -Fq 'DELETE /api/plans/{id}' "${plans_api}"
grep -Fq 'NPopconfirm' "${plans_view}"
grep -Fq '删除后会清除本系统中的路径和配置' "${plans_view}"

if sed -n '/<template>/,/<\/template>/p' "${panel}" | grep -Eq '动作组合循环次数|到达|前置动作|处理结果'; then
  echo 'F-008 节点侧栏或动作弹窗仍暴露旧动作概念' >&2
  exit 1
fi

echo 'F-008 节点动作与循环结构检查通过'
