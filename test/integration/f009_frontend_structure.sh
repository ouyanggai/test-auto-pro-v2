#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
paths_view="${project_root}/web/src/views/PlanPathsView.vue"
preparation_api="${project_root}/web/src/features/path-preparation/api.ts"
preparation_types="${project_root}/web/src/features/path-preparation/types.ts"
execution_types="${project_root}/web/src/features/execution-paths/types.ts"
preparation_service="${project_root}/internal/service/path_preparation.go"
preparation_repository="${project_root}/internal/repository/mysql/path_preparation_repository.go"
plan_types="${project_root}/web/src/features/plans/types.ts"
plan_paths_config_view="${project_root}/web/src/views/PlanPathConfigurationView.vue"
node_config_panel="${project_root}/web/src/features/path-configuration/NodeConfigurationPanel.vue"

grep -Fq 'createPathPreparation' "${preparation_api}"
grep -Fq 'fetchActivePathPreparation' "${preparation_api}"
grep -Fq 'fetchPathPreparationItems' "${preparation_api}"
grep -Fq 'cancelPathPreparation' "${preparation_api}"
grep -Fq 'resumePathPreparation' "${preparation_api}"
grep -Fq 'dataStatus' "${execution_types}"
grep -Fq 'dataDetail' "${execution_types}"
grep -Fq 'PathPreparationJob' "${preparation_types}"
grep -Fq 'currentPath?' "${preparation_types}"
grep -Fq 'processed' "${paths_view}"
grep -Fq 'nodeConfigured' "${paths_view}"
grep -Fq 'dataGenerated' "${paths_view}"
grep -Fq 'needsAttention' "${paths_view}"
grep -Fq '数据需处理' "${paths_view}"
grep -Fq 'selectedRunPathIDs' "${paths_view}"
grep -Fq 'applySelectedConfiguration' "${paths_view}"
grep -Fq 'cancelPathPreparation' "${paths_view}"
grep -Fq 'resumePathPreparation' "${paths_view}"
grep -Fq ':items="visiblePaths"' "${paths_view}"
grep -Fq ':item-size="PREPARATION_PATH_ITEM_SIZE"' "${paths_view}"
if grep -Eq 'preparationListRef|unconfiguredHighlight|已选择未配置路径|path-preparation__item--attention|scrollTo\(\{ index \}\)' "${paths_view}"; then
  echo 'F-009 勾选路径不得触发滚动、高亮或定位提示' >&2
  exit 1
fi
grep -Fq 'findFirstUnrunnableExecutionPath' "${project_root}/web/src/features/execution-paths/logic.ts"
grep -Fq 'executionPathRunReadiness' "${project_root}/web/src/features/execution-paths/logic.ts"
grep -Fq 'preparationJob.currentPath' "${paths_view}"
grep -Fq "preparationJob.value = null" "${paths_view}"
grep -Fq "message.success(\`批量配置完成" "${paths_view}"
if grep -Eq 'preparationItems|preparationNextCursor|refreshPreparationItems|loadMorePreparationItems|path-preparation__job-items' "${paths_view}"; then
  echo 'F-009 页面进度区不得重新堆叠任务明细' >&2
  exit 1
fi
grep -Fq "export type PlanStatus = 'not_started' | 'running' | 'completed'" "${plan_types}"
grep -Fq "const planMutable = computed(() => plan.value?.status === 'not_started')" "${paths_view}"
grep -Fq ':read-only="!planMutable"' "${plan_paths_config_view}"
grep -Fq ':form="runtimeForm"' "${plan_paths_config_view}"
grep -Fq ':disabled="readOnly"' "${node_config_panel}"
if grep -Eq 'planStatusLabels|pending_configuration|开始运行|可运行' "${paths_view}" "${plan_types}"; then
  echo 'F-009 计划详情不得保留旧计划状态或配置状态标签' >&2
  exit 1
fi
grep -Fq 'GetMany' "${preparation_service}"
grep -Fq 'FindByPaths' "${preparation_service}"
grep -Fq 'pathPreparationBatchSize = 25' "${preparation_service}"
grep -Fq 'loadPathPreparationAssets' "${preparation_service}"
grep -Fq 'ClaimBatch' "${preparation_repository}"
grep -Fq 'SetCurrent' "${preparation_repository}"
grep -Fq 'ListItems' "${preparation_repository}"
if grep -RInE 'configuration/preset|generate-all|previewAllExecutionPaths|全选运行|配置下一条|一键预设' "${project_root}/internal" "${project_root}/web/src" --exclude-dir=node_modules; then
  echo 'F-009 不得保留已删除的旧同步预设或批量生成入口' >&2
  exit 1
fi

PATHS_VIEW="${paths_view}" node <<'NODE'
const fs = require('node:fs')
const source = fs.readFileSync(process.env.PATHS_VIEW, 'utf8')
const completedBranch = source.slice(source.indexOf("if (refreshedJob.status === 'completed')"), source.indexOf('schedulePreparationPoll()', source.indexOf("if (refreshedJob.status === 'completed')")))
if (!completedBranch.includes('await retryPaths()') || !completedBranch.includes('preparationJob.value = null')) {
  throw new Error('批量任务完成后必须先刷新路径再收起进度区')
}
if (completedBranch.indexOf('await retryPaths()') > completedBranch.indexOf('preparationJob.value = null')) {
  throw new Error('批量任务进度区在路径状态刷新前被收起')
}
NODE

echo 'F-009 进度、定位、三态与只读边界结构检查通过'
