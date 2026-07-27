<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NDatePicker,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NRadio,
  NRadioGroup,
  NSelect,
} from 'naive-ui'
import { useRouter } from 'vue-router'

import { hasPlanFormErrors, shouldShowMaxConcurrency, validatePlanForm } from '../features/plans/logic'
import { accountOptions, flowSourceOptions, getTargetFlowOptions } from '../features/plans/mock'
import type { PlanFormErrors, PlanFormValue } from '../features/plans/types'

const router = useRouter()
const form = reactive<PlanFormValue>({
  name: '',
  accountId: null,
  flowSource: null,
  flowId: null,
  runMode: 'serial',
  maxConcurrency: null,
  scheduledAt: null,
})
const formErrors = ref<PlanFormErrors>({})
const feedback = ref<{ type: 'error' | 'success'; message: string } | null>(null)

const targetFlowOptions = computed(() => getTargetFlowOptions(form.accountId, form.flowSource))
const showMaxConcurrency = computed(() => shouldShowMaxConcurrency(form.runMode))
const targetFlowPlaceholder = computed(() => {
  if (!form.accountId) return '请先选择真实账号'
  if (!form.flowSource) return '请先选择唯一流程来源'
  return '请选择目标流程'
})

watch([() => form.accountId, () => form.flowSource], () => {
  form.flowId = null
})

watch(() => form.runMode, (runMode) => {
  form.maxConcurrency = runMode === 'parallel' ? 2 : null
})

watch(form, () => {
  formErrors.value = {}
  feedback.value = null
}, { deep: true })

function submitPrototype() {
  const errors = validatePlanForm(form)
  formErrors.value = errors

  if (hasPlanFormErrors(errors)) {
    feedback.value = { type: 'error', message: '请先完成表单中的必填项和条件字段。' }
    return
  }

  feedback.value = {
    type: 'success',
    message: '静态原型已完成校验，真实创建将在后续功能接入。',
  }
}
</script>

<template>
  <section class="new-plan-page">
    <n-button text type="primary" class="back-button" @click="router.push('/plans')">
      返回测试计划
    </n-button>

    <header class="page-heading">
      <h1>新建计划</h1>
      <p>先确定计划来源和运行方式。当前页面只验证静态交互，不会保存或创建真实计划。</p>
    </header>

    <n-form
      class="plan-form"
      label-placement="left"
      label-align="right"
      :label-width="150"
      :show-require-mark="true"
      @submit.prevent="submitPrototype"
    >
      <n-form-item
        label="计划名称"
        required
        :validation-status="formErrors.name ? 'error' : undefined"
        :feedback="formErrors.name"
      >
        <n-input v-model:value="form.name" maxlength="60" show-count placeholder="例如：采购申请主流程回归" />
      </n-form-item>

      <n-form-item
        label="真实账号"
        required
        :validation-status="formErrors.accountId ? 'error' : undefined"
        :feedback="formErrors.accountId"
      >
        <n-select v-model:value="form.accountId" filterable clearable :options="accountOptions" placeholder="请选择 mock 真实账号" />
      </n-form-item>

      <n-form-item
        label="唯一流程来源"
        required
        :validation-status="formErrors.flowSource ? 'error' : undefined"
        :feedback="formErrors.flowSource"
      >
        <n-select v-model:value="form.flowSource" clearable :options="flowSourceOptions" placeholder="请选择流程来源" />
      </n-form-item>

      <n-form-item
        label="目标流程"
        required
        :validation-status="formErrors.flowId ? 'error' : undefined"
        :feedback="formErrors.flowId"
      >
        <n-select
          v-model:value="form.flowId"
          filterable
          clearable
          :disabled="!form.accountId || !form.flowSource"
          :options="targetFlowOptions"
          :placeholder="targetFlowPlaceholder"
        />
      </n-form-item>

      <n-form-item label="运行方式" required>
        <n-radio-group v-model:value="form.runMode">
          <n-radio value="serial">串行</n-radio>
          <n-radio value="parallel">并行</n-radio>
        </n-radio-group>
      </n-form-item>

      <n-form-item
        v-if="showMaxConcurrency"
        label="并行最大并发数"
        required
        :validation-status="formErrors.maxConcurrency ? 'error' : undefined"
        :feedback="formErrors.maxConcurrency"
      >
        <n-input-number v-model:value="form.maxConcurrency" :min="2" :max="20" :precision="0" />
      </n-form-item>

      <n-form-item label="定时时间">
        <n-date-picker
          v-model:value="form.scheduledAt"
          type="datetime"
          clearable
          placeholder="不设置则手动启动"
        />
      </n-form-item>

      <div class="form-actions">
        <n-button type="primary" attr-type="submit">创建并选择路径</n-button>
      </div>
    </n-form>

    <n-alert
      v-if="feedback"
      class="form-feedback"
      :type="feedback.type"
      :title="feedback.type === 'success' ? '校验完成' : '请检查表单'"
      closable
      @close="feedback = null"
    >
      {{ feedback.message }}
    </n-alert>
  </section>
</template>

<style scoped>
.new-plan-page {
  width: min(100%, 920px);
  min-width: 0;
}

.back-button {
  margin-bottom: 24px;
}

.page-heading {
  margin-bottom: 36px;
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
  width: min(100%, 760px);
}

.form-actions {
  padding-left: 150px;
  margin-top: 8px;
}

.form-feedback {
  width: min(100%, 760px);
  margin-top: 24px;
}
</style>
