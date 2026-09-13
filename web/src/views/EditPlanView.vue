<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import {
  NButton,
  NDatePicker,
  NForm,
  NFormItemGi,
  NGrid,
  NInput,
  NInputNumber,
  NRadioButton,
  NRadioGroup,
  NSwitch,
  NText,
  useMessage,
  type FormInst,
  type FormRules,
} from 'naive-ui'
import { useRoute, useRouter } from 'vue-router'

import { fetchPlan, PlanApiError, updatePlan, type CreatePlanRequest } from '../features/plans/persistence'
import type { FlowSource, PlanRunMode, PersistedPlan } from '../features/plans/types'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const formRef = ref<FormInst | null>(null)
const loading = ref(true)
const saving = ref(false)
const loadError = ref('')
let loadController: AbortController | null = null

const form = reactive<{
  name: string
  account: string
  accountDisplayName: string
  flowSource: FlowSource
  targetObjectId: string
  targetObjectName: string
  runMode: PlanRunMode
  maxConcurrency: number | null
  scheduleEnabled: boolean
  scheduledAt: number | null
}>({
  name: '', account: '', accountDisplayName: '', flowSource: 'new', targetObjectId: '', targetObjectName: '',
  runMode: 'serial', maxConcurrency: null, scheduleEnabled: false, scheduledAt: null,
})
const planID = computed(() => String(route.params.id || ''))
const showMaxConcurrency = computed(() => form.runMode === 'parallel' && form.scheduleEnabled)
const rules = computed<FormRules>(() => ({
  name: { required: true, trigger: ['input', 'blur'], message: '请输入计划名称' },
  account: { required: true, trigger: ['input', 'blur'], message: '请输入账号' },
  targetObjectId: { required: true, trigger: ['input', 'blur'], message: '请输入流程标识' },
  targetObjectName: { required: true, trigger: ['input', 'blur'], message: '请输入流程名称' },
  maxConcurrency: showMaxConcurrency.value
    ? { required: true, type: 'number', trigger: ['change', 'blur'], min: 2, max: 20, message: '并行最大并发数应为 2 至 20' }
    : [],
  scheduledAt: form.scheduleEnabled
    ? { required: true, type: 'number', trigger: ['change', 'blur'], message: '请选择启动时间' }
    : [],
}))

function fillForm(plan: PersistedPlan) {
  form.name = plan.name
  form.account = plan.account
  form.accountDisplayName = plan.accountDisplayName
  form.flowSource = plan.flowSource
  form.targetObjectId = plan.targetObjectId
  form.targetObjectName = plan.targetObjectName
  form.runMode = plan.runMode
  form.maxConcurrency = plan.maxConcurrency
  form.scheduleEnabled = plan.scheduledAt !== null
  form.scheduledAt = plan.scheduledAt ? new Date(plan.scheduledAt).getTime() : null
}

// 保持与新建计划一致：并行计划即使关闭定时启动也带有默认并发值，避免隐藏字段导致保存请求不完整。
watch(() => form.runMode, (mode) => {
  if (mode === 'serial') form.maxConcurrency = null
  else if (form.maxConcurrency === null) form.maxConcurrency = 2
})

async function loadPlan() {
  if (!planID.value) {
    loadError.value = '计划标识缺失'
    loading.value = false
    return
  }
  loadController?.abort()
  const controller = new AbortController()
  loadController = controller
  loading.value = true
  loadError.value = ''
  try {
    fillForm(await fetchPlan(planID.value, controller.signal))
  }
  catch (error) {
    if (controller.signal.aborted) return
    loadError.value = error instanceof PlanApiError ? error.message : '暂时无法读取计划，请重试'
  }
  finally {
    if (loadController === controller) loading.value = false
  }
}

function buildPayload(): CreatePlanRequest {
  return {
    name: form.name.trim(),
    account: form.account.trim(),
    accountDisplayName: form.accountDisplayName.trim(),
    flowSource: form.flowSource,
    targetObjectId: form.targetObjectId.trim(),
    targetObjectName: form.targetObjectName.trim(),
    runMode: form.runMode,
    maxConcurrency: form.runMode === 'parallel' ? (form.maxConcurrency ?? 2) : null,
    scheduledAt: form.scheduleEnabled && form.scheduledAt !== null ? new Date(form.scheduledAt).toISOString() : null,
  }
}

async function savePlan() {
  if (!formRef.value || saving.value || !planID.value) return
  try {
    await formRef.value.validate()
  }
  catch {
    message.error('请检查标红的必填项')
    return
  }
  saving.value = true
  try {
    const updated = await updatePlan(planID.value, buildPayload(), new AbortController().signal)
    fillForm(updated)
    message.success('计划已保存')
    await router.push('/plans')
  }
  catch (error) {
    message.error(error instanceof PlanApiError ? error.message : '暂时无法保存计划，请重试')
  }
  finally {
    saving.value = false
  }
}

onMounted(() => { void loadPlan() })
onBeforeUnmount(() => loadController?.abort())
</script>

<template>
  <section class="edit-plan-page">
    <div class="back-bar"><n-button text type="primary" @click="router.push('/plans')">返回测试计划</n-button></div>
    <div v-if="loadError" class="edit-plan-error" role="alert">
      <n-text type="error">{{ loadError }}</n-text>
      <n-button text type="primary" @click="loadPlan">重试</n-button>
    </div>
    <div v-else-if="loading" class="edit-plan-loading">正在读取计划……</div>
    <div v-else class="form-content">
      <header class="page-heading">
        <h1>编辑计划</h1>
        <p>计划配置可以反复调整；保存后只影响后续任务，已经创建的任务记录不会被修改。</p>
      </header>
      <n-form ref="formRef" :model="form" :rules="rules" label-placement="top" @submit.prevent="savePlan">
        <n-grid :cols="24" :x-gap="24">
          <n-form-item-gi span="12" path="name" label="计划名称"><n-input v-model:value="form.name" maxlength="60" show-count /></n-form-item-gi>
          <n-form-item-gi span="12" path="account" label="发起账号"><n-input v-model:value="form.account" maxlength="100" /></n-form-item-gi>
          <n-form-item-gi span="12" path="accountDisplayName" label="账号显示名称"><n-input v-model:value="form.accountDisplayName" maxlength="100" /></n-form-item-gi>
          <n-form-item-gi span="12" path="flowSource" label="流程来源">
            <n-radio-group v-model:value="form.flowSource"><n-radio-button value="new">流程模板</n-radio-button><n-radio-button value="started">已发流程</n-radio-button><n-radio-button value="pending">待发流程</n-radio-button></n-radio-group>
          </n-form-item-gi>
          <n-form-item-gi span="12" path="targetObjectId" label="流程标识"><n-input v-model:value="form.targetObjectId" /></n-form-item-gi>
          <n-form-item-gi span="12" path="targetObjectName" label="流程名称"><n-input v-model:value="form.targetObjectName" /></n-form-item-gi>
          <n-form-item-gi span="24" :show-label="false" :show-feedback="false">
            <n-text depth="3">修改发起账号、流程来源或流程标识后，已有路径会标记为需重新核对；历史任务不受影响。</n-text>
          </n-form-item-gi>
          <n-form-item-gi span="12" path="runMode" label="计划间运行方式">
            <n-radio-group v-model:value="form.runMode"><n-radio-button value="serial">串行</n-radio-button><n-radio-button value="parallel">并行</n-radio-button></n-radio-group>
          </n-form-item-gi>
          <n-form-item-gi span="12" label="定时启动" :show-feedback="false">
            <div class="schedule-switch"><n-switch v-model:value="form.scheduleEnabled" /><n-text depth="3">{{ form.scheduleEnabled ? '将在指定时间启动' : '关闭时由用户手动启动' }}</n-text></div>
          </n-form-item-gi>
          <n-form-item-gi v-if="form.scheduleEnabled" span="12" path="scheduledAt" label="启动时间"><n-date-picker v-model:value="form.scheduledAt" class="full-width-control" type="datetime" clearable /></n-form-item-gi>
          <n-form-item-gi v-if="showMaxConcurrency" span="12" path="maxConcurrency" label="路径最大并发数"><n-input-number v-model:value="form.maxConcurrency" class="full-width-control" :min="2" :max="20" :precision="0" /></n-form-item-gi>
          <n-form-item-gi span="24" :show-label="false" :show-feedback="false"><div class="form-actions"><n-button type="primary" attr-type="submit" :loading="saving">保存计划</n-button></div></n-form-item-gi>
        </n-grid>
      </n-form>
    </div>
  </section>
</template>

<style scoped>
.edit-plan-page { width: 100%; min-width: 0; }
.back-bar { min-height: 44px; padding: 8px 0 12px; }
.form-content { width: min(100%, 960px); margin: 0 auto; padding: 8px 0 40px; }
.page-heading { margin-bottom: 32px; }
.page-heading h1 { margin: 0 0 10px; font-size: 28px; font-weight: 600; line-height: 1.25; }
.page-heading p { margin: 0; color: var(--n-text-color-2); line-height: 1.7; }
.edit-plan-loading, .edit-plan-error { padding: 80px 0; text-align: center; }
.edit-plan-error { display: flex; justify-content: center; gap: 16px; align-items: center; }
.schedule-switch { display: flex; align-items: center; gap: 12px; min-height: 34px; }
.full-width-control { width: 100%; }
.form-actions { display: flex; justify-content: flex-end; padding-top: 8px; }
</style>
