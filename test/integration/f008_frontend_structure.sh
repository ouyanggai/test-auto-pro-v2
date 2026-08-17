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
grep -Fq 'action-row__actions' "${panel}"
grep -Fq 'aria-label="删除动作"' "${panel}"
grep -Fq '已配置的动作' "${panel}"
if grep -Fq '准备情况' "${panel}"; then exit 1; fi
grep -Fq 'action-select' "${panel}"
grep -Fq 'action-count' "${panel}"
grep -Fq 'person-controls' "${panel}"
grep -Fq 'actionDraft' "${panel}"
grep -Fq 'actionDraft.value = next' "${panel}"
grep -Fq '@click="openActionEditor"' "${panel}"
grep -Fq '@click="saveActionEditor"' "${panel}"
if grep -Eq 'action-base-row|beginner-hint|保存只更新当前节点|循环配置|cycleEditorOpen|cycle-fixed-note' "${panel}"; then exit 1; fi
if grep -Eq 'transfer_approver|transpond|转办|转发|移交' "${panel}"; then exit 1; fi
grep -Fq 'actionCycles' "${view}"
grep -Fq '纳入本次测试' "${view}"
grep -Fq '已准备 {{ configuration.preparation.preparedNodes }} 个节点' "${view}"
grep -Fq '>一键预设</n-button>' "${view}"
grep -Fq '>复制已保存循环</n-button>' "${view}"
grep -Fq '当前路径' "${view}"
grep -Fq '已选路径' "${view}"
grep -Fq '全部兼容路径' "${view}"
grep -Fq '确认应用' "${view}"
grep -Fq 'PathConfigActionCycle' "${model}"
grep -Fq 'restart_from_initiator' "${workspace}"
grep -Fq 'redo_previous_task' "${workspace}"
grep -Fq '循环与重复次数总在写入前按最新路径复验' "${workspace}"
grep -Fq 'configuration/selection' "${api}"
grep -Fq 'configuration/preset/preview' "${api}"
grep -Fq 'configuration/preset/apply' "${api}"
grep -Fq 'configuration/cycles/copy' "${api}"
grep -Fq 'SaveSelection' "${workspace}"
grep -Fq 'DELETE /api/plans/{id}' "${plans_api}"
grep -Fq 'NPopconfirm' "${plans_view}"
grep -Fq '删除后会清除本系统中的路径和配置' "${plans_view}"
grep -Fq 'Strategy: "random"' "${project_root}/internal/analyzer/path_config_plan.go"
grep -Fq 'deterministicPathConfigPeople' "${project_root}/internal/analyzer/path_config_plan.go"

if sed -n '/<template>/,/<\/template>/p' "${panel}" | grep -Eq '动作组合循环次数|前置动作|处理结果|动作计划'; then
  echo 'F-008 节点侧栏或动作弹窗仍暴露旧动作概念' >&2
  exit 1
fi

if grep -RInE 'actionPlan|actionPlans|PathConfigActionPlan|rollbackTargets|combinationCount|addSignNodes|requiresTarget' "${project_root}/internal" "${project_root}/web/src" --exclude-dir=node_modules; then
  echo 'F-008 仍保留旧动作计划契约' >&2
  exit 1
fi

echo 'F-008 节点动作与循环结构检查通过'
