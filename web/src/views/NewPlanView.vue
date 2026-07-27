<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import {
  NButton,
  NDatePicker,
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
import { getMockFlowCandidates } from '../features/plans/mock'
import { flowSourceLabels, isFlowSourceAvailable } from '../features/plans/selection'
import type { AccountVerificationState, FlowCandidate, FlowSource, PlanFormValue } from '../features/plans/types'

const router = useRouter()
const message = useMessage()
const formRef = ref<FormInst | null>(null)
const accountItemRef = ref<FormItemInst | null>(null)
const selectionItemRef = ref<FormItemInst | null>(null)
const concurrencyItemRef = ref<FormItemInst | null>(null)
const scheduledAtItemRef = ref<FormItemInst | null>(null)

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

const verificationState = ref<AccountVerificationState>('idle')
const verifiedAccount = ref('')
let verificationVersion = 0
const selectionVersion = ref(0)

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
const candidates = computed<FlowCandidate[]>(() => (
  accountVerified.value ? getMockFlowCandidates(form.flowSource, verifiedAccount.value) : []
))
const selectedCandidateKey = computed(() => {
  if (form.flowSource === 'new') return form.templateId
  if (form.flowSource === 'started') return form.submittedFlowId
  return form.dueFlowId
})
const candidateRequestKey = computed(() => (
  `${selectionVersion.value}:${verifiedAccount.value}:${form.flowSource}`
))
const accountStatusCopy = computed(() => {
  if (verificationState.value === 'verifying') return '正在校验本地输入，不会请求真实平台…'
  if (accountVerified.value) return `已完成“${verifiedAccount.value}”的本地静态验证，未登录真实平台。`
  if (verificationState.value === 'invalid') return '账号已编辑，原验证失效，请重新验证。'
  return '本轮仅验证界面联动，不会登录真实平台或生成 SID。'
})
const accountStatusType = computed(() => (accountVerified.value ? 'success' : verificationState.value === 'invalid' ? 'warning' : undefined))

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
      message: '请输入真实账号',
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
    ? { required: true, type: 'number', trigger: ['change', 'blur'], message: '请选择启动时间' }
    : [],
}))

function clearFlowSelections() {
  form.templateId = null
  form.submittedFlowId = null
  form.dueFlowId = null
  selectionVersion.value += 1
}

watch(() => form.account, async () => {
  verificationVersion += 1
  if (verificationState.value !== 'idle') verificationState.value = 'invalid'
  verifiedAccount.value = ''
  if (form.flowSource !== 'new') form.flowSource = 'new'
  clearFlowSelections()
  await nextTick()
  selectionItemRef.value?.restoreValidation()
})

watch(() => form.flowSource, async (source) => {
  if (!isFlowSourceAvailable(source, accountVerified.value)) {
    form.flowSource = 'new'
    return
  }
  clearFlowSelections()
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

async function verifyAccount() {
  try {
    await accountItemRef.value?.validate('blur')
  }
  catch {
    return
  }

  const account = form.account.trim()
  const version = ++verificationVersion
  verificationState.value = 'verifying'
  await nextTick()
  await new Promise<void>((resolve) => window.requestAnimationFrame(() => resolve()))
  if (version !== verificationVersion || account !== form.account.trim()) return
  verifiedAccount.value = account
  verificationState.value = 'verified'
  accountItemRef.value?.restoreValidation()
  message.success('本地静态验证完成，未登录真实平台')
}

async function selectCandidate(key: string) {
  if (form.flowSource === 'new') form.templateId = key
  else if (form.flowSource === 'started') form.submittedFlowId = key
  else form.dueFlowId = key
  await nextTick()
  selectionItemRef.value?.restoreValidation()
}

async function submitPrototype() {
  if (!formRef.value) return
  try {
    await formRef.value.validate()
    message.success('静态原型已完成校验，真实创建将在后续功能接入。')
  }
  catch {
    message.error('请检查标红的必填项')
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
        <p>先验证账号并选择对应流程。当前页面只演示静态交互，不会登录目标平台或保存计划。</p>
      </header>

      <n-form
        ref="formRef"
        class="plan-form"
        :model="form"
        :rules="rules"
        label-placement="top"
        require-mark-placement="right-hanging"
        :show-feedback="true"
        @submit.prevent="submitPrototype"
      >
        <n-grid :cols="24" :x-gap="24">
          <n-form-item-gi span="12" path="name" label="计划名称" first>
            <n-input v-model:value="form.name" maxlength="60" show-count placeholder="例如：采购申请主流程回归" />
          </n-form-item-gi>

          <n-form-item-gi ref="accountItemRef" span="12" path="account" label="真实账号" first>
            <div class="account-control">
              <n-input-group>
                <n-input
                  v-model:value="form.account"
                  clearable
                  placeholder="请输入目标平台真实账号"
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
            <n-date-picker
              v-model:value="form.scheduledAt"
              class="full-width-control"
              type="datetime"
              clearable
              placeholder="请选择启动时间"
            />
          </n-form-item-gi>

          <n-form-item-gi
            v-if="showMaxConcurrency"
            ref="concurrencyItemRef"
            span="12"
            path="maxConcurrency"
            label="并行最大并发数"
            first
          >
            <n-input-number
              v-model:value="form.maxConcurrency"
              class="full-width-control"
              :min="2"
              :max="20"
              :precision="0"
            />
          </n-form-item-gi>

          <n-form-item-gi
            v-if="form.flowSource === 'new'"
            ref="selectionItemRef"
            span="24"
            path="templateId"
            label="流程模板"
            first
          >
            <flow-candidate-list
              v-if="accountVerified"
              source="new"
              :items="candidates"
              :selected-key="selectedCandidateKey"
              :request-key="candidateRequestKey"
              @select="selectCandidate"
            />
            <n-empty v-else class="verification-empty" description="请先验证账号，再加载该账号可见的流程模板" />
          </n-form-item-gi>

          <n-form-item-gi
            v-else-if="form.flowSource === 'started'"
            ref="selectionItemRef"
            span="24"
            path="submittedFlowId"
            label="已发流程"
            first
          >
            <flow-candidate-list
              source="started"
              :items="candidates"
              :selected-key="selectedCandidateKey"
              :request-key="candidateRequestKey"
              @select="selectCandidate"
            />
          </n-form-item-gi>

          <n-form-item-gi
            v-else
            ref="selectionItemRef"
            span="24"
            path="dueFlowId"
            label="待发流程"
            first
          >
            <flow-candidate-list
              source="pending"
              :items="candidates"
              :selected-key="selectedCandidateKey"
              :request-key="candidateRequestKey"
              @select="selectCandidate"
            />
          </n-form-item-gi>

          <n-form-item-gi span="24" :show-label="false" :show-feedback="false">
            <div class="form-actions">
              <n-button type="primary" attr-type="submit">创建并选择路径</n-button>
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
  z-index: 3;
  width: max-content;
  padding: 8px 0;
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
  min-height: 180px;
  padding-top: 42px;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  width: 100%;
  padding-top: 8px;
}
</style>
