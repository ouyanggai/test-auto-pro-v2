#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
form_file="${project_root}/web/src/views/NewPlanView.vue"
candidate_file="${project_root}/web/src/features/plans/FlowCandidateList.vue"
api_file="${project_root}/web/src/features/plans/api.ts"
remote_file="${project_root}/web/src/features/plans/remote.ts"
styles_file="${project_root}/web/src/styles.css"

grep -Fq 'verifyTargetAccount' "${form_file}"
grep -Fq 'fetchTargetCandidates' "${form_file}"
grep -Fq 'new AbortController()' "${form_file}"
grep -Fq 'candidateRequestVersion' "${form_file}"
grep -Fq 'isCurrentRemoteRequest' "${form_file}"
grep -Fq 'mergeCandidatePages' "${form_file}"
grep -Fq 'searchDebouncer.schedule(query)' "${form_file}"
grep -Fq 'invalidateVerifiedAccount(apiError.message)' "${form_file}"
grep -Fq "code === 'TARGET_SESSION_EXPIRED'" "${remote_file}"
grep -Fq 'REMOTE_SEARCH_DEBOUNCE_MS = 250' "${remote_file}"

grep -Fq '<n-virtual-list' "${candidate_file}"
grep -Fq ':item-size="CANDIDATE_ITEM_SIZE"' "${candidate_file}"
grep -Fq "emit('queryChange', value)" "${candidate_file}"
grep -Fq "emit('loadMore')" "${candidate_file}"
grep -Fq "emit('retry')" "${candidate_file}"
grep -Fq 'min-height: 574px' "${candidate_file}"
grep -Fq 'height: 480px' "${candidate_file}"
grep -Fq 'height: 96px' "${candidate_file}"
grep -Fq "candidate.kind === 'template') return ''" "${project_root}/web/src/features/plans/presentation.ts"
grep -Fq 'templateCompanyName' "${candidate_file}"
grep -Fq 'type="info"' "${candidate_file}"
grep -Fq 'candidate.companyName' "${project_root}/web/src/features/plans/presentation.ts"
grep -Fq "candidate.remark.trim() || '暂无备注'" "${project_root}/web/src/features/plans/presentation.ts"
grep -Fq "candidate.formTemplateCount" "${project_root}/web/src/features/plans/presentation.ts"
grep -Fq 'candidate-row__detail--remark' "${candidate_file}"
grep -Fq -- '-webkit-line-clamp: 2' "${candidate_file}"
grep -Fq '@media (prefers-reduced-motion: reduce)' "${candidate_file}"

grep -Fq '/api/target/accounts/verify' "${api_file}"
grep -Fq '/api/target/flow-templates' "${api_file}"
grep -Fq '/api/target/flow-instances' "${api_file}"
grep -Fq "params.source === 'started' ? 'submitted' : 'due'" "${api_file}"
grep -Fq 'companyName: item.companyName' "${api_file}"

! grep -Fq '编码 ${candidate.code}' "${project_root}/web/src/features/plans/presentation.ts"
! grep -Fq 'candidate.code,' "${project_root}/web/src/features/plans/selection.ts"

! grep -Fq 'getMockFlowCandidates' "${form_file}"
! grep -RInF 'getMockFlowCandidates' "${project_root}/web/src" >/dev/null
! grep -RInE '(localStorage|sessionStorage).*([Ss][Ii][Dd]|password|AES)' "${project_root}/web/src" >/dev/null

grep -Fq 'position: sticky' "${form_file}"
grep -Fq 'min-height: 574px' "${form_file}"
grep -Fq 'overflow: hidden' "${form_file}"
grep -Fq 'overflow: hidden' "${styles_file}"

if grep -RInE '真实目标平台|目标平台真实账号|不会登录真实平台|本地静态验证|未登录真实平台|目标平台账号|该账号可见的真实流程|静态原型已完成校验' "${form_file}" "${candidate_file}" "${api_file}" >/dev/null; then
	printf 'F-002 页面仍包含过期技术证明式文案\n' >&2
	exit 1
fi
