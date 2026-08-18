#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
paths_view="${project_root}/web/src/views/PlanPathsView.vue"
configuration_view="${project_root}/web/src/views/PlanPathConfigurationView.vue"
paths_api="${project_root}/web/src/features/execution-paths/api.ts"

grep -Fq '新增路径' "${paths_view}"
grep -Fq '>编辑路径</n-button>' "${paths_view}"
grep -Fq 'NVirtualList' "${paths_view}"
grep -Fq 'fetchExecutionPath(planID.value, path.id)' "${paths_view}"
grep -Fq 'startPathGeneration' "${paths_view}"
grep -Fq 'fetchPathGeneration' "${paths_view}"
grep -Fq 'resetPageScroll' "${paths_view}"
grep -Fq 'scroll-snap-type: y mandatory' "${paths_view}"
grep -Fq '一键配置' "${paths_view}"

grep -Fq 'fetchExecutionPath(planID.value, pathID.value, controller.signal)' "${configuration_view}"
grep -Fq 'analyzeExecutionPath(storedGraph, storedPath.choices)' "${configuration_view}"
grep -Fq 'path-configuration-page__error-content' "${configuration_view}"
grep -Fq 'path-configuration-page__cycle-body' "${configuration_view}"

grep -Fq '/path-generations' "${paths_api}"
if grep -RInE 'GenerateAll|generate-all|一键生成全部路径|全选运行|配置下一条|previewAllExecutionPaths|applySelectedPreset' \
  "${paths_view}" "${configuration_view}" "${paths_api}"; then
  echo 'F-005 仍保留已经删除的旧路径接口或旧入口' >&2
  exit 1
fi

echo 'F-005 当前路径摘要、按需详情和滚动行为检查通过'
