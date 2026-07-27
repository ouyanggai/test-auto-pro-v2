#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
router_file="${project_root}/web/src/router/index.ts"
list_file="${project_root}/web/src/views/PlansView.vue"
form_file="${project_root}/web/src/views/NewPlanView.vue"
app_file="${project_root}/web/src/App.vue"
styles_file="${project_root}/web/src/styles.css"

grep -Fq "path: '/plans'" "${router_file}"
grep -Fq "path: '/plans/new'" "${router_file}"
grep -Fq "import('../views/PlansView.vue')" "${router_file}"
grep -Fq "import('../views/NewPlanView.vue')" "${router_file}"

grep -Fq '<n-data-table' "${list_file}"
grep -Fq ':scroll-x="1370"' "${list_file}"
grep -Fq "router.push('/plans/new')" "${list_file}"
for column_name in 计划名称 流程名称 发起账号 路径数量 运行方式 定时时间 计划状态 最近运行结果 操作; do
  grep -Fq "${column_name}" "${list_file}"
done
for filter_contract in 'v-model:value="filters.name"' 'v-model:value="filters.status"' '@click="clearFilters"'; do
  grep -Fq "${filter_contract}" "${list_file}"
done

grep -Fq "router.push('/plans')" "${form_file}"
grep -Fq '创建并选择路径' "${form_file}"
grep -Fq '静态原型已完成校验，真实创建将在后续功能接入。' "${form_file}"
grep -Fq "message.error('请检查标红的必填项')" "${form_file}"
grep -Fq "message.success('静态原型已完成校验，真实创建将在后续功能接入。')" "${form_file}"
grep -Fq 'v-if="showMaxConcurrency"' "${form_file}"
grep -Fq 'watch([() => form.accountId, () => form.flowSource]' "${form_file}"
grep -Fq 'targetFlowItemRef.value?.restoreValidation()' "${form_file}"
grep -Fq 'concurrencyItemRef.value?.restoreValidation()' "${form_file}"
for field_name in 计划名称 真实账号 唯一流程来源 目标流程 运行方式 并行最大并发数 定时时间; do
  grep -Fq "${field_name}" "${form_file}"
done

for form_contract in \
  'ref="formRef"' \
  ':model="form"' \
  ':rules="rules"' \
  'label-placement="top"' \
  'require-mark-placement="right-hanging"' \
  ':show-feedback="true"' \
  '<n-grid :cols="24" :x-gap="24">' \
  'span="12" path="name"' \
  'span="12" path="accountId"' \
  'span="12" path="flowSource"' \
  'span="12" path="flowId"' \
  'span="12" path="runMode"' \
  'span="12" path="scheduledAt"' \
  'span="24" :show-label="false" :show-feedback="false"'; do
  grep -Fq "${form_contract}" "${form_file}"
done

for rule_contract in \
  "trigger: ['input', 'blur']" \
  "trigger: ['change', 'blur']" \
  'flowId: targetFlowEnabled.value' \
  ': []' \
  "new Error('并行最大并发数应为 2 至 20')"; do
  grep -Fq "${rule_contract}" "${form_file}"
done

grep -Fq 'NMessageProvider' "${app_file}"
grep -Fq '<n-message-provider>' "${app_file}"
grep -Fq 'useMessage' "${form_file}"

if grep -Eq '<n-alert|formErrors|validation-status=|:feedback=|validatePlanForm|hasPlanFormErrors|show-require-mark' "${form_file}"; then
  printf 'F-001 表单不应保留页面级错误块或手写校验状态\n' >&2
  exit 1
fi

FORM_FILE="${form_file}" node <<'NODE'
const fs = require('node:fs')
const source = fs.readFileSync(process.env.FORM_FILE, 'utf8')
const rulesSource = source.match(/const rules = computed<FormRules>\(\(\) => \(\{([\s\S]*?)\n\}\)\)/)?.[1]

if (!rulesSource) throw new Error('未找到 Naive UI FormRules 定义')
if (rulesSource.includes('scheduledAt:')) throw new Error('可选定时时间不应配置必填规则')
if (!/flowId:\s*targetFlowEnabled\.value[\s\S]*?:\s*\[\]/.test(rulesSource)) {
  throw new Error('前置条件缺失时目标流程规则应为空，不产生第三个红错')
}
if (!source.includes('first>')) throw new Error('每个必填表项应启用 first，仅显示首个错误')
NODE

grep -Fq 'overflow: hidden;' "${styles_file}"
grep -Fq '.app-main > .n-layout-scroll-container' "${styles_file}"
grep -Fq 'overflow-y: auto;' "${styles_file}"

if grep -REq 'fetch\(|axios|/api/' \
  "${project_root}/web/src/features/plans" \
  "${list_file}" \
  "${form_file}"; then
  printf 'F-001 静态原型不应访问后端 API\n' >&2
  exit 1
fi
