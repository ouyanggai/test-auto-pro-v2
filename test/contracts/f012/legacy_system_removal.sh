#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "${project_root}"

for path in \
  internal/formdata/constraint_ir.go \
  internal/formdata/generator.go \
  internal/service/path_preparation.go \
  internal/service/template_catalog.go \
  internal/service/run_input_preflight.go \
  internal/adapter/target/form_submission.go \
  form-runtime/src/runtime/vueFieldBridge.js; do
  if [[ -e "${path}" ]]; then
    printf '旧 F-012 代码仍存在：%s\n' "${path}" >&2
    exit 1
  fi
done

legacy_pattern='ConstraintIR|constraint_ir|TemplateCatalog|template_catalog|PathPreparation|path-preparation|RunInputPreflight|run-input/preflight|generatedValues|generatedFieldPaths|manualOverridePaths|setGeneratedData|draft_save|approve_pass|reject_no_pass|vueFieldBridge|ruleVersion'
legacy_result="$(mktemp)"
trap 'rm -f "${legacy_result}"' EXIT
if grep -RInE "${legacy_pattern}" cmd internal web/src form-runtime/src --exclude-dir=node_modules --exclude='*.sum' --exclude='*.map' >"${legacy_result}" 2>/dev/null; then
  cat "${legacy_result}" >&2
  exit 1
fi

for route in \
  '/configuration/form/generate' \
  '/configuration/cycles' \
  '/run-input/preflight' \
  '/path-preparations' \
  '/template-rules'; do
  if grep -RInF "${route}" cmd internal web/src form-runtime/src --exclude-dir=node_modules --exclude='*.sum' >/dev/null 2>&1; then
    printf '旧 F-012 路由仍被运行代码引用：%s\n' "${route}" >&2
    exit 1
  fi
done

printf 'F-012 旧体系符号、入口和桥接清理检查通过\n'
