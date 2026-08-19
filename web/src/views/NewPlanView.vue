<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import {
  NButton,
  NDatePicker,
  NDivider,
  NEmpty,
  NForm,
  NFormItemGi,
  NGrid,
  NInput,
  NInputGroup,
  NInputNumber,
  NRadioButton,
  NRadioGroup,
  NSwitch,
  NText,
  useMessage,
  type FormInst,
  type FormItemInst,
  type FormItemRule,
  type FormRules,
} from 'naive-ui'
import { useRouter } from 'vue-router'

import FlowCandidateList from '../features/plans/FlowCandidateList.vue'
import { fetchTargetCandidates, TargetApiError, verifyTargetAccount } from '../features/plans/api'
import { buildCreatePlanRequest, createPlan, PlanApiError } from '../features/plans/persistence'
import {
  createDebouncedRunner,
  invalidatesVerification,
  isCurrentRemoteRequest,
  mergeCandidatePages,
  retryPageFor,
  type RemoteRequestIdentity,
} from '../features/plans/remote'
import {
  calculateNearestScrollDelta,
  flowSelectionLabels,
  flowSourceLabels,
  isFlowSourceAvailable,
  resolvePostSelectionGuidance,
} from '../features/plans/selection'
import type {
  AccountVerificationState,
  FlowCandidate,
  FlowSource,
  PlanFormValue,
  VerifiedTargetAccount,
} from '../features/plans/types'

const router = useRouter()
const message = useMessage()
const formRef = ref<FormInst | null>(null)
const accountItemRef = ref<FormItemInst | null>(null)
const selectionItemRef = ref<FormItemInst | null>(null)
const concurrencyItemRef = ref<FormItemInst | null>(null)
const scheduledAtItemRef = ref<FormItemInst | null>(null)
const selectionRegionRef = ref<HTMLElement | null>(null)
const scheduledAtGuideRef = ref<HTMLElement | null>(null)
const concurrencyGuideRef = ref<HTMLElement | null>(null)
const submitGuideRef = ref<HTMLElement | null>(null)
const candidateListRef = ref<{
  getSearchElement: () => HTMLInputElement | null
  focusSearch: () => void
} | null>(null)

const form = reactive<PlanFormValue>({
  name: '',
  account: '',
  flowSource: 'new',
  templateId: null,
  submittedFlowId: null,
  dueFlowId: null,
  runMode: 'serial',
  maxConcurrency: null,
  scheduleEnabled: false,
  scheduledAt: null,
})
const selectedCandidate = ref<FlowCandidate | null>(null)
const creationLoading = ref(false)
const creationKey = ref<string | null>(null)
let creationController: AbortController | null = null

const verificationState = ref<AccountVerificationState>('idle')
const verificationError = ref('')
const verifiedAccount = ref('')
const verifiedSummary = ref<VerifiedTargetAccount | null>(null)
let verificationVersion = 0
let verificationController: AbortController | null = null

const candidates = ref<FlowCandidate[]>([])
const candidateLoading = ref(false)
const candidateLoadingMore = ref(false)
const candidateError = ref('')
const candidateHasMore = ref(false)
const candidateTotal = ref(0)
const candidatePage = ref(0)
const candidateQuery = ref('')
const candidateFailedPage = ref<number | null>(null)
const selectionVersion = ref(0)
let candidateRequestVersion = 0
let candidateController: AbortController | null = null
const searchDebouncer = createDebouncedRunner((query: string) => {
  void loadCandidatePage(1, true, query)
})

const accountVerified = computed(() => (
  verificationState.value === 'verified'
  && verifiedAccount.value === form.account.trim()
))
const showMaxConcurrency = computed(() => form.runMode === 'parallel')
const sourceOptions = computed(() => (
  (Object.entries(flowSourceLabels) as Array<[FlowSource, string]>).map(([value, label]) => ({
    value,
    label,
    disabled: !isFlowSourceAvailable(value, accountVerified.value),
  }))
))
const selectedCandidateKey = computed(() => {
  return selectedCandidate.value?.key ?? null
})
const candidateRequestKey = computed(() => (
  `${selectionVersion.value}:${verifiedAccount.value}:${form.flowSource}`
))
const selectionLabel = computed(() => flowSelectionLabels[form.flowSource])
const selectionPath = computed(() => {
  if (form.flowSource === 'new') return 'templateId'
  if (form.flowSource === 'started') return 'submittedFlowId'
  return 'dueFlowId'
})
const accountStatusCopy = computed(() => {
	if (verificationState.value === 'verifying') return '正在验证账号…'
  if (accountVerified.value) {
    const identity = [verifiedSummary.value?.displayName, verifiedSummary.value?.companyName].filter(Boolean).join(' · ')
		return identity ? `账号已验证：${identity}` : `账号“${verifiedAccount.value}”已验证。`
  }
  if (verificationState.value === 'failed') return verificationError.value || '账号验证失败，请重试。'
  if (verificationState.value === 'invalid') return '账号已编辑，原验证失效，请重新验证。'
	return '验证账号后可读取流程。'
})
const accountStatusType = computed(() => {
  if (accountVerified.value) return 'success'
  if (verificationState.value === 'invalid') return 'warning'
  if (verificationState.value === 'failed') return 'error'
  return undefined
})

let guidanceVersion = 0
let pendingSelectionGuidance: number | null = null

function nextFrame() {
  return new Promise<void>((resolve) => window.requestAnimationFrame(() => resolve()))
}

function prefersReducedMotion(): boolean {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

function getMainScrollContainer(): HTMLElement | null {
  return document.querySelector<HTMLElement>('.app-main > .n-layout-scroll-container')
}

async function guideTo(target: 'selection' | 'scheduledAt' | 'maxConcurrency' | 'submit', version: number, focusSearch = false) {
  await nextTick()
  await nextFrame()
  if (version !== guidanceVersion) return

  const element = target === 'selection'
    ? candidateListRef.value?.getSearchElement() ?? selectionRegionRef.value
    : target === 'scheduledAt'
      ? scheduledAtGuideRef.value
      : target === 'maxConcurrency'
        ? concurrencyGuideRef.value
        : submitGuideRef.value
  const container = getMainScrollContainer()
  if (!element || !container) return

  const delta = calculateNearestScrollDelta(
    element.getBoundingClientRect().top,
    element.getBoundingClientRect().bottom,
    container.getBoundingClientRect().top,
    container.getBoundingClientRect().bottom,
  )
  if (delta === 0) return

  container.scrollBy({ top: delta, behavior: prefersReducedMotion() ? 'auto' : 'smooth' })
  if (focusSearch) candidateListRef.value?.focusSearch()
}

function requestSelectionGuidance() {
  const version = ++guidanceVersion
  pendingSelectionGuidance = version
}

function handleSelectionContentEntered() {
  const version = pendingSelectionGuidance
  pendingSelectionGuidance = null
  if (version === null) return
  void guideTo('selection', version, true)
}

function requestPostSelectionGuidance() {
  const version = ++guidanceVersion
  const nextTarget = resolvePostSelectionGuidance(form)
  void guideTo(nextTarget, version)
}

const maxConcurrencyRules = computed<FormItemRule[]>(() => {
  if (form.runMode !== 'parallel') return []
  return [
    {
      required: true,
      type: 'number',
      trigger: ['change', 'blur'],
      message: '请输入并行最大并发数',
    },
    {
      trigger: ['change', 'blur'],
      validator: (_rule: FormItemRule, value: number | null) => {
        if (typeof value !== 'number' || value < 2 || value > 20) {
          return new Error('并行最大并发数应为 2 至 20')
        }
        return true
      },
    },
  ]
})

const rules = computed<FormRules>(() => ({
  name: {
    required: true,
    trigger: ['input', 'blur'],
    message: '请输入计划名称',
  },
  account: [
    {
      required: true,
      trigger: ['input', 'blur'],
		message: '请输入账号',
    },
    {
      trigger: 'account-verification',
      validator: () => accountVerified.value || new Error('请先验证当前账号'),
    },
  ],
  flowSource: {
    required: true,
    trigger: 'change',
    message: '请选择流程来源',
  },
  templateId: accountVerified.value && form.flowSource === 'new'
    ? { required: true, trigger: 'change', message: '请选择流程模板' }
    : [],
  submittedFlowId: accountVerified.value && form.flowSource === 'started'
    ? { required: true, trigger: 'change', message: '请选择已发流程' }
    : [],
  dueFlowId: accountVerified.value && form.flowSource === 'pending'
    ? { required: true, trigger: 'change', message: '请选择待发流程' }
    : [],
  maxConcurrency: maxConcurrencyRules.value,
  scheduledAt: form.scheduleEnabled
    ? [
        { required: true, type: 'number', trigger: ['change', 'blur'], message: '请选择启动时间' },
        {
          trigger: ['change', 'blur'],
          validator: (_rule: FormItemRule, value: number | null) => (
            typeof value === 'number' && value > Date.now()
              ? true
              : new Error('启动时间必须晚于当前时间')
          ),
        },
      ]
    : [],
}))

function clearFlowSelections() {
  form.templateId = null
  form.submittedFlowId = null
  form.dueFlowId = null
  selectedCandidate.value = null
  selectionVersion.value += 1
}

function cancelCandidateRequest() {
  candidateController?.abort()
  candidateController = null
  candidateRequestVersion += 1
  candidateLoading.value = false
  candidateLoadingMore.value = false
}

function clearCandidateState() {
  cancelCandidateRequest()
  candidates.value = []
  candidateError.value = ''
  candidateHasMore.value = false
  candidateTotal.value = 0
  candidatePage.value = 0
  candidateFailedPage.value = null
}

function invalidateVerifiedAccount(errorMessage: string) {
  searchDebouncer.cancel()
  clearCandidateState()
  verifiedAccount.value = ''
  verifiedSummary.value = null
  verificationState.value = 'failed'
  verificationError.value = errorMessage
  if (form.flowSource !== 'new') form.flowSource = 'new'
  clearFlowSelections()
}

function currentRequestIdentity(): RemoteRequestIdentity {
  return {
    version: candidateRequestVersion,
    account: verifiedAccount.value,
    source: form.flowSource,
    query: candidateQuery.value,
  }
}

async function loadCandidatePage(page: number, reset: boolean, query: string) {
  if (!accountVerified.value) return
  candidateController?.abort()
  const controller = new AbortController()
  candidateController = controller
  candidateQuery.value = query
  const requestIdentity: RemoteRequestIdentity = {
    version: ++candidateRequestVersion,
    account: verifiedAccount.value,
    source: form.flowSource,
    query,
  }
  candidateError.value = ''
  candidateFailedPage.value = null
  if (reset) {
    candidates.value = []
    candidatePage.value = 0
    candidateTotal.value = 0
    candidateHasMore.value = false
    candidateLoading.value = true
  }
  else {
    candidateLoadingMore.value = true
  }

  try {
    const result = await fetchTargetCandidates({
      account: requestIdentity.account,
      source: requestIdentity.source as FlowSource,
      query: requestIdentity.query,
      page,
      pageSize: 20,
      signal: controller.signal,
    })
    if (!isCurrentRemoteRequest(currentRequestIdentity(), requestIdentity) || result.account !== requestIdentity.account) return
    candidates.value = reset ? result.items : mergeCandidatePages(candidates.value, result.items)
    candidatePage.value = result.page
    candidateTotal.value = result.total
    candidateHasMore.value = result.hasMore && result.items.length > 0
  }
  catch (error) {
    if (controller.signal.aborted || !isCurrentRemoteRequest(currentRequestIdentity(), requestIdentity)) return
		const apiError = error instanceof TargetApiError ? error : new TargetApiError('读取流程失败，请重试')
    if (invalidatesVerification(apiError.code)) {
      invalidateVerifiedAccount(apiError.message)
      message.error(apiError.message)
      return
    }
    candidateError.value = apiError.message
    candidateFailedPage.value = page
  }
  finally {
    if (isCurrentRemoteRequest(currentRequestIdentity(), requestIdentity)) {
      candidateLoading.value = false
      candidateLoadingMore.value = false
      candidateController = null
    }
  }
}

function handleCandidateQueryChange(query: string) {
  candidateQuery.value = query
  candidateController?.abort()
  candidateController = null
  candidateRequestVersion += 1
  candidateLoading.value = false
  candidateLoadingMore.value = false
	 candidateHasMore.value = false
  candidateError.value = ''
  candidateFailedPage.value = null
  searchDebouncer.schedule(query)
}

function loadMoreCandidates() {
	if (!accountVerified.value || candidateLoading.value || candidateLoadingMore.value || candidateError.value || !candidateHasMore.value) return
  void loadCandidatePage(candidatePage.value + 1, false, candidateQuery.value)
}

function retryCandidates() {
  if (!accountVerified.value) return
  const page = retryPageFor(candidates.value, candidatePage.value, candidateFailedPage.value)
  void loadCandidatePage(page, page === 1, candidateQuery.value)
}

watch(() => form.account, async () => {
  verificationController?.abort()
  verificationController = null
  verificationVersion += 1
  guidanceVersion += 1
  pendingSelectionGuidance = null
  searchDebouncer.cancel()
  if (verificationState.value !== 'idle') verificationState.value = 'invalid'
  verificationError.value = ''
  verifiedAccount.value = ''
  verifiedSummary.value = null
  if (form.flowSource !== 'new') form.flowSource = 'new'
  candidateQuery.value = ''
  clearCandidateState()
  clearFlowSelections()
  await nextTick()
  selectionItemRef.value?.restoreValidation()
})

watch(() => form.flowSource, async (source) => {
  if (!isFlowSourceAvailable(source, accountVerified.value)) {
    form.flowSource = 'new'
    return
  }
  searchDebouncer.cancel()
  candidateQuery.value = ''
  clearCandidateState()
  clearFlowSelections()
  if (accountVerified.value) {
    requestSelectionGuidance()
    void loadCandidatePage(1, true, '')
  }
  await nextTick()
  selectionItemRef.value?.restoreValidation()
})

watch(() => form.runMode, async (runMode) => {
  concurrencyItemRef.value?.restoreValidation()
  form.maxConcurrency = runMode === 'parallel' ? 2 : null
  await nextTick()
  concurrencyItemRef.value?.restoreValidation()
})

watch(() => form.scheduleEnabled, async (enabled) => {
  scheduledAtItemRef.value?.restoreValidation()
  if (!enabled) form.scheduledAt = null
  await nextTick()
  scheduledAtItemRef.value?.restoreValidation()
})

watch(form, () => {
  if (!creationLoading.value) creationKey.value = null
}, { deep: true })

async function verifyAccount() {
  try {
    await accountItemRef.value?.validate('blur')
  }
  catch {
    return
  }

  verificationController?.abort()
  const controller = new AbortController()
  verificationController = controller
  const account = form.account.trim()
  const version = ++verificationVersion
  verificationState.value = 'verifying'
  verificationError.value = ''
  try {
    const summary = await verifyTargetAccount(account, controller.signal)
    if (controller.signal.aborted || version !== verificationVersion || account !== form.account.trim()) return
    verifiedAccount.value = account
    verifiedSummary.value = summary
    verificationState.value = 'verified'
    accountItemRef.value?.restoreValidation()
    candidateQuery.value = ''
    clearCandidateState()
    clearFlowSelections()
    requestSelectionGuidance()
		message.success('账号验证成功')
    void loadCandidatePage(1, true, '')
  }
  catch (error) {
    if (controller.signal.aborted || version !== verificationVersion) return
    const apiError = error instanceof TargetApiError ? error : new TargetApiError('账号验证失败，请重试')
	 invalidateVerifiedAccount(apiError.message)
    message.error(apiError.message)
  }
  finally {
    if (version === verificationVersion) verificationController = null
  }
}

async function selectCandidate(key: string) {
  const candidate = candidates.value.find((item) => item.key === key)
  if (!candidate) return
  selectedCandidate.value = candidate
  if (candidate.kind === 'template') form.templateId = candidate.templateId
  else if (candidate.kind === 'submitted') form.submittedFlowId = candidate.id
  else form.dueFlowId = candidate.flowInstanceId
  await nextTick()
  selectionItemRef.value?.restoreValidation()
  requestPostSelectionGuidance()
}

async function submitPlan() {
  if (!formRef.value || creationLoading.value) return
  try {
    await formRef.value.validate()
  }
  catch {
    message.error('请检查标红的必填项')
    return
  }
  if (!verifiedSummary.value || !selectedCandidate.value) {
    message.error('请重新选择当前流程')
    return
  }

  const idempotencyKey = creationKey.value || crypto.randomUUID()
  creationKey.value = idempotencyKey
  creationController?.abort()
  const controller = new AbortController()
  creationController = controller
  creationLoading.value = true
  try {
    const plan = await createPlan(
      buildCreatePlanRequest(form, verifiedSummary.value, selectedCandidate.value),
      idempotencyKey,
      controller.signal,
    )
    creationKey.value = null
    message.success('计划已创建')
    await router.push(`/plans/${plan.id}/paths`)
  }
  catch (error) {
    if (controller.signal.aborted) return
    const apiError = error instanceof PlanApiError ? error : new PlanApiError('暂时无法创建计划，请重试')
    message.error(apiError.message)
  }
  finally {
    if (creationController === controller) creationController = null
    creationLoading.value = false
  }
}
</script>

<template>
  <section class="new-plan-page">
    <div class="back-bar">
      <n-button text type="primary" @click="router.push('/plans')">返回测试计划</n-button>
    </div>

    <div class="form-content">
      <header class="page-heading">
        <h1>新建计划</h1>
		<p>验证账号并选择流程，创建后计划状态为未运行。</p>
      </header>

      <n-form
        ref="formRef"
        class="plan-form"
        :model="form"
        :rules="rules"
        label-placement="top"
        require-mark-placement="right-hanging"
        :show-feedback="true"
        @submit.prevent="submitPlan"
      >
        <n-grid :cols="24" :x-gap="24">
          <n-form-item-gi span="12" path="name" label="计划名称" first>
            <n-input v-model:value="form.name" maxlength="60" show-count placeholder="例如：采购申请主流程回归" />
          </n-form-item-gi>

		  <n-form-item-gi ref="accountItemRef" span="12" path="account" label="账号" first>
            <div class="account-control">
              <n-input-group>
                <n-input
                  v-model:value="form.account"
                  clearable
				  placeholder="请输入账号"
                  @keydown.enter.prevent="verifyAccount"
                />
                <n-button
                  attr-type="button"
                  :loading="verificationState === 'verifying'"
                  @click="verifyAccount"
                >
                  验证账号
                </n-button>
              </n-input-group>
              <n-text class="account-status" :type="accountStatusType" depth="3" aria-live="polite">
                {{ accountStatusCopy }}
              </n-text>
            </div>
          </n-form-item-gi>

          <n-form-item-gi span="12" path="flowSource" label="流程来源" first>
            <div class="source-control">
              <n-radio-group v-model:value="form.flowSource">
                <n-radio-button
                  v-for="option in sourceOptions"
                  :key="option.value"
                  :value="option.value"
                  :disabled="option.disabled"
                >
                  {{ option.label }}
                </n-radio-button>
              </n-radio-group>
              <n-text v-if="!accountVerified" depth="3">验证账号后可选择“已发”或“待发”。</n-text>
            </div>
          </n-form-item-gi>

          <n-form-item-gi span="12" path="runMode" label="运行方式" first>
            <n-radio-group v-model:value="form.runMode">
              <n-radio-button value="serial">串行</n-radio-button>
              <n-radio-button value="parallel">并行</n-radio-button>
            </n-radio-group>
          </n-form-item-gi>

          <n-form-item-gi span="12" label="定时启动" :show-feedback="false">
            <div class="schedule-switch">
              <n-switch v-model:value="form.scheduleEnabled" />
              <n-text depth="3">{{ form.scheduleEnabled ? '将在指定时间启动' : '关闭时由用户手动启动' }}</n-text>
            </div>
          </n-form-item-gi>

          <n-form-item-gi
            v-if="form.scheduleEnabled"
            ref="scheduledAtItemRef"
            span="12"
            path="scheduledAt"
            label="启动时间"
            first
          >
            <div ref="scheduledAtGuideRef" class="full-width-control">
              <n-date-picker
                v-model:value="form.scheduledAt"
                class="full-width-control"
                type="datetime"
                clearable
                placeholder="请选择启动时间"
              />
            </div>
          </n-form-item-gi>

          <n-form-item-gi
            v-if="showMaxConcurrency"
            ref="concurrencyItemRef"
            span="12"
            path="maxConcurrency"
            label="并行最大并发数"
            first
          >
            <div ref="concurrencyGuideRef" class="full-width-control">
              <n-input-number
                v-model:value="form.maxConcurrency"
                class="full-width-control"
                :min="2"
                :max="20"
                :precision="0"
              />
            </div>
          </n-form-item-gi>

          <n-form-item-gi
            span="24"
            :show-label="false"
            :show-feedback="false"
          >
            <n-divider class="selection-divider" title-placement="left">选择流程</n-divider>
          </n-form-item-gi>

          <n-form-item-gi
            ref="selectionItemRef"
            span="24"
            :path="selectionPath"
            :label="selectionLabel"
            first
          >
            <div ref="selectionRegionRef" class="selection-shell">
              <transition name="selection-content" mode="out-in" @after-enter="handleSelectionContentEntered">
                <flow-candidate-list
                  v-if="accountVerified"
                  ref="candidateListRef"
                  :key="candidateRequestKey"
                  :source="form.flowSource"
                  :items="candidates"
                  :selected-key="selectedCandidateKey"
                  :request-key="candidateRequestKey"
				  :account-name="verifiedSummary?.displayName || verifiedAccount"
				  :loading="candidateLoading"
				  :loading-more="candidateLoadingMore"
				  :error="candidateError"
				  :has-more="candidateHasMore"
				  :total="candidateTotal"
                  @select="selectCandidate"
				  @query-change="handleCandidateQueryChange"
				  @load-more="loadMoreCandidates"
				  @retry="retryCandidates"
                />
                <div v-else key="unverified" class="selection-placeholder">
				  <n-empty description="请先验证账号，再读取可见流程" />
                </div>
              </transition>
            </div>
          </n-form-item-gi>

          <n-form-item-gi span="24" :show-label="false" :show-feedback="false">
            <div ref="submitGuideRef" class="form-actions">
              <n-button type="primary" attr-type="submit" :loading="creationLoading" :disabled="creationLoading">
                创建并选择路径
              </n-button>
            </div>
          </n-form-item-gi>
        </n-grid>
      </n-form>
    </div>
  </section>
</template>

<style scoped>
.new-plan-page {
  width: 100%;
  min-width: 0;
  background-color: inherit;
}

.back-bar {
  position: sticky;
  top: 0;
  z-index: 2;
  width: 100%;
  min-height: 44px;
  padding: 8px 0 12px;
  background-color: inherit;
}

.form-content {
  width: min(100%, 960px);
  margin: 0 auto;
  padding: 8px 0 40px;
}

.page-heading {
  margin-bottom: 32px;
}

.page-heading h1 {
  margin: 0 0 10px;
  font-size: 28px;
  font-weight: 600;
  line-height: 1.25;
}

.page-heading p {
  margin: 0;
  color: var(--n-text-color-2);
  line-height: 1.7;
}

.plan-form {
  width: 100%;
}

.account-control,
.source-control {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
  min-width: 0;
}

.account-status {
  min-height: 20px;
}

.source-control .n-text,
.account-status {
  font-size: 12px;
}

.schedule-switch {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 34px;
}

.full-width-control {
  width: 100%;
}

.verification-empty {
  width: 100%;
}

.selection-divider {
  margin: 12px 0 20px;
}

.selection-shell {
  display: grid;
  width: 100%;
	min-height: 574px;
  overflow: hidden;
}

.selection-shell > * {
  width: 100%;
}

.selection-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
	min-height: 574px;
}

.selection-content-enter-active,
.selection-content-leave-active {
  transition: opacity 160ms ease, transform 160ms ease;
}

.selection-content-enter-from,
.selection-content-leave-to {
  opacity: 0;
  transform: translateY(4px);
}

@media (prefers-reduced-motion: reduce) {
  .selection-content-enter-active,
  .selection-content-leave-active {
    transition: none;
  }

  .selection-content-enter-from,
  .selection-content-leave-to {
    transform: none;
  }
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  width: 100%;
  padding-top: 8px;
}
</style>
