#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
catalog_view="${project_root}/web/src/views/TemplateRuleCatalogView.vue"
catalog_api="${project_root}/web/src/features/template-catalog/api.ts"
router="${project_root}/web/src/router/index.ts"
settings="${project_root}/web/src/views/SettingsView.vue"
frame="${project_root}/web/src/features/path-configuration/FormRuntimeFrame.vue"
runtime="${project_root}/form-runtime/src/App.vue"
host_page="${project_root}/form-runtime/src/HostVuePage.vue"
host_registry="${project_root}/form-runtime/src/runtime/hostVuePages.js"

grep -Fq "/settings/template-rules" "${router}"
grep -Fq "模板规则目录" "${settings}"
grep -Fq "createTemplateRuleAnalysis" "${catalog_api}"
grep -Fq "fetchTemplateRuleSummary" "${catalog_api}"
grep -Fq "fetchTemplateRuleCatalog" "${catalog_api}"
grep -Fq "增量分析" "${catalog_view}"
grep -Fq "全量重分析" "${catalog_view}"
grep -Fq "重试失败项" "${catalog_view}"
grep -Fq "已处理 {{ job.processed }} / {{ job.total }}" "${catalog_view}"
grep -Fq '<n-pagination' "${catalog_view}"
grep -Fq 'fetchTemplateRuleCatalog(pageNumber.value, pageSize' "${catalog_view}"
grep -Fq "vue_custom" "${frame}"
grep -Fq "vuePage: props.form.vuePage" "${frame}"
grep -Fq "renderType === 'vue_custom'" "${runtime}"
grep -Fq "resolveHostVuePage" "${host_page}"
grep -Fq 'instance.$children' "${host_page}"
grep -Fq "setDescendantsDisabled" "${host_page}"
grep -Fq "ContractReview" "${host_registry}"
grep -Fq "CompanyBudget" "${host_registry}"
grep -Fq "TravelExpenseForm" "${host_registry}"
if grep -Eq '<input|<select|<textarea' "${runtime}" "${host_page}"; then
  echo 'F-010 Vue 页面不得在本项目重画普通表单控件，必须加载宿主真实组件' >&2
  exit 1
fi
grep -Fq "当前数据源无可用记录" "${project_root}/internal/service/path_config_workspace.go"
if grep -Fq "组件不支持" "${catalog_view}"; then
  echo 'F-010 设置页不得把已识别业务组件泛化为组件不支持' >&2
  exit 1
fi
if grep -Eq 'NoForm|contract_review|合同评审' "${catalog_view}" "${catalog_api}"; then
  echo 'F-010 前端不得增加单个流程专用适配分支' >&2
  exit 1
fi

PROJECT_ROOT="${project_root}" node <<'NODE'
const fs = require('node:fs')
const path = require('node:path')
const root = process.env.PROJECT_ROOT
const settings = fs.readFileSync(path.join(root, 'form-runtime/runtime-source/src/store/modules/settings.js'), 'utf8')
const registry = fs.readFileSync(path.join(root, 'form-runtime/src/runtime/hostVuePages.js'), 'utf8')
const registered = new Set([...settings.matchAll(/component\s*:\s*['"]([A-Za-z0-9_]+)['"]/g)].map(match => match[1]).filter(Boolean))
const configRoot = path.join(root, 'form-runtime/runtime-source/src/components/NoFormFLow/config')
for (const file of fs.readdirSync(configRoot).filter(name => name.endsWith('Config.js'))) registered.add(file.replace(/Config\.js$/, ''))
const missing = [...registered].filter(name => !new RegExp(`\\b${name}\\b`).test(registry))
if (missing.length) throw new Error(`宿主 Vue 页面注册表缺少：${missing.join('、')}`)
NODE

echo 'F-010 规则目录、Vue 页面运行时和失败边界结构检查通过'
