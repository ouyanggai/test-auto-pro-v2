#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
paths_view="${project_root}/web/src/views/PlanPathsView.vue"
preparation_api="${project_root}/web/src/features/path-preparation/api.ts"
preparation_types="${project_root}/web/src/features/path-preparation/types.ts"
execution_types="${project_root}/web/src/features/execution-paths/types.ts"
preparation_service="${project_root}/internal/service/path_preparation.go"
preparation_repository="${project_root}/internal/repository/mysql/path_preparation_repository.go"

grep -Fq 'createPathPreparation' "${preparation_api}"
grep -Fq 'fetchActivePathPreparation' "${preparation_api}"
grep -Fq 'fetchPathPreparationItems' "${preparation_api}"
grep -Fq 'cancelPathPreparation' "${preparation_api}"
grep -Fq 'resumePathPreparation' "${preparation_api}"
grep -Fq 'dataStatus' "${execution_types}"
grep -Fq 'dataDetail' "${execution_types}"
grep -Fq 'PathPreparationJob' "${preparation_types}"
grep -Fq 'processed' "${paths_view}"
grep -Fq 'nodeConfigured' "${paths_view}"
grep -Fq 'dataGenerated' "${paths_view}"
grep -Fq 'needsAttention' "${paths_view}"
grep -Fq '数据需处理' "${paths_view}"
grep -Fq 'selectedRunPathIDs' "${paths_view}"
grep -Fq 'applySelectedConfiguration' "${paths_view}"
grep -Fq 'cancelPathPreparation' "${paths_view}"
grep -Fq 'resumePathPreparation' "${paths_view}"
grep -Fq 'path-preparation__job-items' "${paths_view}"
grep -Fq ':items="visiblePaths"' "${paths_view}"
grep -Fq ':item-size="PREPARATION_PATH_ITEM_SIZE"' "${paths_view}"
grep -Fq 'reason' "${paths_view}"
grep -Fq 'GetMany' "${preparation_service}"
grep -Fq 'FindByPaths' "${preparation_service}"
grep -Fq 'pathPreparationBatchSize = 25' "${preparation_service}"
grep -Fq 'loadPathPreparationAssets' "${preparation_service}"
grep -Fq 'ClaimBatch' "${preparation_repository}"
grep -Fq 'ListItems' "${preparation_repository}"
if grep -RInE 'configuration/preset|generate-all|previewAllExecutionPaths|全选运行|配置下一条|一键预设' "${project_root}/internal" "${project_root}/web/src" --exclude-dir=node_modules; then
  echo 'F-009 不得保留已删除的旧同步预设或批量生成入口' >&2
  exit 1
fi

echo 'F-009 批量准备与独立数据状态结构检查通过'
