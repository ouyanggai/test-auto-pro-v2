#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
manifest="${project_root}/form-runtime/sync-manifest.json"
runtime_main="${project_root}/form-runtime/src/main.js"
upstream_main="${project_root}/form-runtime/runtime-source/src/main.js"
vue_config="${project_root}/form-runtime/vue.config.js"
runtime_package="${project_root}/form-runtime/package.json"
maintenance_view="${project_root}/web/src/views/FormRuntimeMaintenanceView.vue"
router="${project_root}/web/src/router/index.ts"
settings="${project_root}/web/src/views/SettingsView.vue"

grep -Fq '"repository": "rsh-flow-components"' "${manifest}"
grep -Fq '"sourceBranch": "master"' "${manifest}"
if grep -Fq '"sourceHead"' "${manifest}"; then
  echo 'F-007 不得把历史来源 HEAD 永久编译进同步清单' >&2
  exit 1
fi
grep -Fq '"target": "runtime-source/src"' "${manifest}"
grep -Fq '"source": "scripts"' "${manifest}"
grep -Fq '"source": "sync-manifest.json"' "${manifest}"
grep -Fq '"protectedLocalPaths"' "${manifest}"
grep -Fq '".npmrc"' "${manifest}"

test -f "${project_root}/form-runtime/runtime-source/src/views/GroupApproveManage/Submitted/components/OtherSteps2.vue"
test -f "${project_root}/form-runtime/runtime-source/src/components/Custom/components/PersonMulSelect/index.vue"
test -f "${project_root}/form-runtime/runtime-source/src/lib/vue-form-making/dist/FormMaking.common.js"
test -f "${project_root}/form-runtime/runtime-source/scripts/sync.js"
test -f "${project_root}/form-runtime/runtime-source/sync-manifest.json"
test -f "${project_root}/form-runtime/src/runtime/requestPolicy.js"
if test -e "${project_root}/form-runtime/runtime-source/.npmrc"; then
  echo 'F-007 实际运行源码不得复制含凭证的 .npmrc' >&2
  exit 1
fi
if test -e "${project_root}/form-runtime/upstream" || test -e "${project_root}/form-runtime/vendor" || test -e "${project_root}/form-runtime/src/runtime/targetComponents.js"; then
  echo 'F-007 不得保留 upstream、vendor 或空组件表的平行运行时' >&2
  exit 1
fi
grep -Fq "import '@runtime/main.js'" "${runtime_main}"
grep -Fq "FormMaking.common" "${upstream_main}"
grep -Fq "custom-upload-excel" "${upstream_main}"
grep -Fq "person-mulSelect" "${upstream_main}"
grep -Fq "FORM_RUNTIME_SOURCE_DIR" "${vue_config}"
grep -Fq "runtime-health-plugin" "${vue_config}"
grep -Fq "runtime-source" "${vue_config}"
grep -Fq 'vue-cli-service serve --host 127.0.0.1 --port 19001' "${runtime_package}"
grep -Fq 'vue-cli-service build' "${runtime_package}"
grep -Fq 'node scripts/sync.js' "${runtime_package}"
grep -Fq '19001/form-runtime/' "${project_root}/web/src/features/path-configuration/FormRuntimeFrame.vue"
grep -Fq 'runtime-health.json' "${project_root}/cmd/server/main.go"

grep -Fq "/settings/form-runtime" "${router}"
grep -Fq "表单运行时维护" "${settings}"
grep -Fq "一键同步并更新运行时" "${maintenance_view}"
grep -Fq "固定来源" "${maintenance_view}"
grep -Fq "恢复结果" "${maintenance_view}"
grep -Fq "在线日志" "${maintenance_view}"
if grep -Eq '<(n-)?input|v-model.*(path|branch|command)' "${maintenance_view}"; then
  echo 'F-007 维护页不得提供任意来源、分支或命令输入' >&2
  exit 1
fi

grep -Fq 'form-runtime-sync:' "${project_root}/Makefile"
grep -Fq 'form-runtime-status:' "${project_root}/Makefile"
grep -Fq '/api/form-runtime-maintenance/jobs' "${project_root}/scripts/form-runtime-maintenance.sh"

for stage in INSPECT SYNC SYNC_CHECK BUILD RESTART VERIFY COMPLETED; do
  grep -Fq "${stage}" "${project_root}/internal/formruntimemaintenance/model.go"
done
grep -Fq "fencing_token" "${project_root}/internal/repository/mysql/migrations/010_create_form_runtime_sync_jobs.sql"
grep -Fq "active_guard" "${project_root}/internal/repository/mysql/migrations/010_create_form_runtime_sync_jobs.sql"

echo 'F-007 完整 rsh-flow-components 入口、动态快照、原生同步、真实健康与维护入口结构检查通过'
