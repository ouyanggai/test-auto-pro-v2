#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
router_file="${project_root}/web/src/router/index.ts"
list_file="${project_root}/web/src/views/PlansView.vue"
form_file="${project_root}/web/src/views/NewPlanView.vue"
candidate_file="${project_root}/web/src/features/plans/FlowCandidateList.vue"
selection_file="${project_root}/web/src/features/plans/selection.ts"
mock_file="${project_root}/web/src/features/plans/mock.ts"
app_file="${project_root}/web/src/App.vue"
styles_file="${project_root}/web/src/styles.css"

grep -Fq "path: '/plans'" "${router_file}"
grep -Fq "path: '/plans/new'" "${router_file}"
grep -Fq "import('../views/PlansView.vue')" "${router_file}"
grep -Fq "import('../views/NewPlanView.vue')" "${router_file}"

grep -Fq '<n-data-table' "${list_file}"
grep -Fq ':scroll-x="1458"' "${list_file}"
grep -Fq "router.push('/plans/new')" "${list_file}"
for column_name in 计划名称 流程名称 发起账号 路径数量 运行方式 定时时间 计划状态 最近运行结果 操作; do
  grep -Fq "${column_name}" "${list_file}"
done
for filter_contract in 'v-model:value="filters.name"' 'v-model:value="filters.status"' '@click="clearFilters"'; do
  grep -Fq "${filter_contract}" "${list_file}"
done

for form_contract in \
  "router.push('/plans')" \
  'ref="formRef"' \
  ':model="form"' \
  ':rules="rules"' \
  'label-placement="top"' \
  'require-mark-placement="right-hanging"' \
  ':show-feedback="true"' \
  '<n-grid :cols="24" :x-gap="24">' \
  'span="12" path="name"' \
  'span="12" path="account"' \
  'label="真实账号"' \
  'v-model:value="form.account"' \
  '验证账号' \
  'label="流程来源"' \
  '创建并选择路径'; do
  grep -Fq "${form_contract}" "${form_file}"
done

for verification_contract in \
  "verificationState.value = 'verifying'" \
  "verificationState.value = 'verified'" \
  "verificationState.value = 'invalid'" \
  '本地静态验证完成，未登录真实平台' \
  "new Error('请先验证当前账号')" \
  "if (form.flowSource !== 'new') form.flowSource = 'new'" \
  'clearFlowSelections()'; do
  grep -Fq "${verification_contract}" "${form_file}"
done

grep -Fq "return source === 'new' || accountVerified" "${selection_file}"
grep -Fq ':disabled="option.disabled"' "${form_file}"
grep -Fq '验证账号后可选择“已发”或“待发”' "${form_file}"

for source_contract in \
  "if (form.flowSource === 'new') return 'templateId'" \
  "if (form.flowSource === 'started') return 'submittedFlowId'" \
  "return 'dueFlowId'" \
  ':path="selectionPath"' \
  ':label="selectionLabel"' \
  ':source="form.flowSource"' \
  'flowSelectionLabels'; do
  grep -Fq "${source_contract}" "${form_file}"
done

for target_field in templateId flowName typeName groupName statusText updateTime; do
  grep -Fq "${target_field}" "${mock_file}"
done
for target_field in id name status createDate currentNodeName currentAuditUserNames; do
  grep -Fq "${target_field}" "${mock_file}"
done
for target_field in flowInstanceId flowInstanceName statusName initiator initiatorDate; do
  grep -Fq "${target_field}" "${mock_file}"
done

for list_contract in \
  'NVirtualList' \
  '<n-virtual-list' \
  ':item-size="84"' \
  '@scroll="handleScroll"' \
  'v-model:value="query"' \
  'requestVersion += 1' \
  'version !== requestVersion' \
  'batchCount.value += 1' \
  '没有匹配的' \
  '正在追加下一批' \
  '已显示全部' \
  'candidate-row--selected'; do
  grep -Fq "${list_contract}" "${candidate_file}"
done

for schedule_contract in \
  '<n-switch v-model:value="form.scheduleEnabled"' \
  'v-if="form.scheduleEnabled"' \
  'path="scheduledAt"' \
  'if (!enabled) form.scheduledAt = null' \
  "message: '请选择启动时间'"; do
  grep -Fq "${schedule_contract}" "${form_file}"
done

grep -Fq 'NDivider' "${form_file}"
grep -Fq '<n-divider class="selection-divider" title-placement="left">选择流程</n-divider>' "${form_file}"
grep -Fq 'class="selection-shell"' "${form_file}"
grep -Fq 'min-height: 348px;' "${form_file}"
grep -Fq '<transition name="selection-content" mode="out-in" @after-enter="handleSelectionContentEntered">' "${form_file}"
grep -Fq '@media (prefers-reduced-motion: reduce)' "${form_file}"
grep -Fq 'transform: translateY(4px);' "${form_file}"
grep -Fq 'defineExpose({ getSearchElement, focusSearch })' "${candidate_file}"
grep -Fq 'searchFieldRef' "${candidate_file}"
grep -Fq "window.matchMedia('(prefers-reduced-motion: reduce)')" "${form_file}"
grep -Fq 'calculateNearestScrollDelta(' "${form_file}"
grep -Fq "container.scrollBy({ top: delta, behavior: prefersReducedMotion() ? 'auto' : 'smooth' })" "${form_file}"
grep -Fq 'if (delta === 0) return' "${form_file}"
grep -Fq 'pendingSelectionGuidance' "${form_file}"
grep -Fq 'requestSelectionGuidance()' "${form_file}"
grep -Fq 'requestPostSelectionGuidance()' "${form_file}"
grep -Fq 'resolvePostSelectionGuidance(form)' "${form_file}"

for back_bar_contract in \
  '.back-bar {' \
  'position: sticky;' \
  'top: 0;' \
  'z-index: 2;' \
  'min-height: 44px;' \
  'background-color: inherit;'; do
  grep -Fq "${back_bar_contract}" "${form_file}"
done

if grep -A8 '^\.back-bar {' "${form_file}" | grep -Eq 'transition:|transform:'; then
  printf 'F-001 吸顶返回入口不应带位移动画\n' >&2
  exit 1
fi

grep -Fq 'margin: 0 auto;' "${form_file}"
grep -Fq 'v-if="showMaxConcurrency"' "${form_file}"
grep -Fq "new Error('并行最大并发数应为 2 至 20')" "${form_file}"
grep -Fq "message.error('请检查标红的必填项')" "${form_file}"
grep -Fq "message.success('静态原型已完成校验，真实创建将在后续功能接入。')" "${form_file}"

grep -Fq 'NMessageProvider' "${app_file}"
grep -Fq '<n-message-provider>' "${app_file}"
grep -Fq 'useMessage' "${form_file}"

if grep -Eq '<n-select|accountOptions|唯一流程来源|目标流程|path="flowId"|form\.flowId|<n-alert|formErrors|validation-status=|:feedback=|show-require-mark' "${form_file}"; then
  printf 'F-001 表单不应保留账号下拉、统一目标流程或页面级错误块\n' >&2
  exit 1
fi

FORM_FILE="${form_file}" node <<'NODE'
const fs = require('node:fs')
const source = fs.readFileSync(process.env.FORM_FILE, 'utf8')
const rulesSource = source.match(/const rules = computed<FormRules>\(\(\) => \(\{([\s\S]*?)\n\}\)\)/)?.[1]

if (!rulesSource) throw new Error('未找到 Naive UI FormRules 定义')
for (const field of ['account:', 'templateId:', 'submittedFlowId:', 'dueFlowId:', 'scheduledAt:']) {
  if (!rulesSource.includes(field)) throw new Error(`缺少 ${field} 的动态规则`)
}
if (!rulesSource.includes("trigger: 'account-verification'")) {
  throw new Error('提交时必须校验账号已完成静态验证')
}
if (!/templateId:[\s\S]*?flowSource === 'new'[\s\S]*?: \[\]/.test(rulesSource)) {
  throw new Error('流程模板规则必须只在账号已验证且来源为新发起时启用')
}
if (!/scheduledAt:\s*form\.scheduleEnabled[\s\S]*?: \[\]/.test(rulesSource)) {
  throw new Error('启动时间规则必须只在定时启动开启时启用')
}
if (!source.includes('first>')) throw new Error('必填表项应启用 first，仅显示首个错误')
NODE

grep -Fq 'overflow: hidden;' "${styles_file}"
grep -Fq '.app-main > .n-layout-scroll-container' "${styles_file}"
grep -Fq 'overflow-y: auto;' "${styles_file}"

grep -Fq '不会登录真实平台或生成 SID' "${form_file}"

if grep -REq 'fetch\(|axios|/api/' \
  "${project_root}/web/src/features/plans" \
  "${list_file}" \
  "${form_file}"; then
  printf 'F-001 静态原型不应访问后端或目标平台\n' >&2
  exit 1
fi
