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
grep -Fq "code === 'TARGET_SESSION_EXPIRED'" "${remote_file}"
grep -Fq 'REMOTE_SEARCH_DEBOUNCE_MS = 250' "${remote_file}"

grep -Fq '<n-virtual-list' "${candidate_file}"
grep -Fq "emit('queryChange', value)" "${candidate_file}"
grep -Fq "emit('loadMore')" "${candidate_file}"
grep -Fq "emit('retry')" "${candidate_file}"
grep -Fq 'min-height: 348px' "${candidate_file}"
grep -Fq '真实目标平台' "${candidate_file}"

grep -Fq '/api/target/accounts/verify' "${api_file}"
grep -Fq '/api/target/flow-templates' "${api_file}"
grep -Fq '/api/target/flow-instances' "${api_file}"
grep -Fq "params.source === 'started' ? 'submitted' : 'due'" "${api_file}"

! grep -Fq 'getMockFlowCandidates' "${form_file}"
! grep -RInF 'getMockFlowCandidates' "${project_root}/web/src" >/dev/null
! grep -RInE '(localStorage|sessionStorage).*([Ss][Ii][Dd]|password|AES)' "${project_root}/web/src" >/dev/null

grep -Fq 'position: sticky' "${form_file}"
grep -Fq 'min-height: 348px' "${form_file}"
grep -Fq 'overflow: hidden' "${form_file}"
grep -Fq 'overflow: hidden' "${styles_file}"
