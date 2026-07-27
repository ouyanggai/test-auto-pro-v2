<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import {
  NButton,
  NDatePicker,
  NForm,
  NFormItemGi,
  NGrid,
  NInput,
  NInputNumber,
  NRadio,
  NRadioGroup,
  NSelect,
  useMessage,
  type FormInst,
  type FormItemInst,
  type FormItemRule,
  type FormRules,
} from 'naive-ui'
import { useRouter } from 'vue-router'

import { accountOptions, flowSourceOptions, getTargetFlowOptions } from '../features/plans/mock'
import type { PlanFormValue } from '../features/plans/types'

const router = useRouter()
const message = useMessage()
const formRef = ref<FormInst | null>(null)
const targetFlowItemRef = ref<FormItemInst | null>(null)
const concurrencyItemRef = ref<FormItemInst | null>(null)
const form = reactive<PlanFormValue>({
  name: '',
  accountId: null,
  flowSource: null,
  flowId: null,
  runMode: 'serial',
  maxConcurrency: null,
  scheduledAt: null,
})

const targetFlowOptions = computed(() => getTargetFlowOptions(form.accountId, form.flowSource))
const targetFlowEnabled = computed(() => Boolean(form.accountId && form.flowSource))
const showMaxConcurrency = computed(() => form.runMode === 'parallel')
const targetFlowPlaceholder = computed(() => {
  if (!form.accountId) return '请先选择真实账号'
  if (!form.flowSource) return '请先选择唯一流程来源'
  return '请选择目标流程'
})

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
  accountId: {
    required: true,
    trigger: ['change', 'blur'],
    message: '请选择真实账号',
  },
  flowSource: {
    required: true,
    trigger: ['change', 'blur'],
    message: '请选择唯一流程来源',
  },
  flowId: targetFlowEnabled.value
    ? {
        required: true,
        trigger: ['change', 'blur'],
        message: '请选择目标流程',
      }
    : [],
  maxConcurrency: maxConcurrencyRules.value,
}))

watch([() => form.accountId, () => form.flowSource], async () => {
  form.flowId = null
  await nextTick()
  targetFlowItemRef.value?.restoreValidation()
})

watch(() => form.runMode, async (runMode) => {
  concurrencyItemRef.value?.restoreValidation()
  form.maxConcurrency = runMode === 'parallel' ? 2 : null
  await nextTick()
  concurrencyItemRef.value?.restoreValidation()
})

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
    <n-button text type="primary" class="back-button" @click="router.push('/plans')">
      返回测试计划
    </n-button>

    <header class="page-heading">
      <h1>新建计划</h1>
      <p>先确定计划来源和运行方式。当前页面只验证静态交互，不会保存或创建真实计划。</p>
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

        <n-form-item-gi span="12" path="accountId" label="真实账号" first>
          <n-select v-model:value="form.accountId" filterable clearable :options="accountOptions" placeholder="请选择 mock 真实账号" />
        </n-form-item-gi>

        <n-form-item-gi span="12" path="flowSource" label="唯一流程来源" first>
          <n-select v-model:value="form.flowSource" clearable :options="flowSourceOptions" placeholder="请选择流程来源" />
        </n-form-item-gi>

        <n-form-item-gi ref="targetFlowItemRef" span="12" path="flowId" label="目标流程" first>
          <n-select
            v-model:value="form.flowId"
            filterable
            clearable
            :disabled="!targetFlowEnabled"
            :options="targetFlowOptions"
            :placeholder="targetFlowPlaceholder"
          />
        </n-form-item-gi>

        <n-form-item-gi span="12" path="runMode" label="运行方式" first>
          <n-radio-group v-model:value="form.runMode">
            <n-radio value="serial">串行</n-radio>
            <n-radio value="parallel">并行</n-radio>
          </n-radio-group>
        </n-form-item-gi>

        <n-form-item-gi span="12" path="scheduledAt" label="定时时间" first>
          <n-date-picker
            v-model:value="form.scheduledAt"
            class="full-width-control"
            type="datetime"
            clearable
            placeholder="不设置则手动启动"
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

        <n-form-item-gi span="24" :show-label="false" :show-feedback="false">
          <div class="form-actions">
            <n-button type="primary" attr-type="submit">创建并选择路径</n-button>
          </div>
        </n-form-item-gi>
      </n-grid>
    </n-form>
  </section>
</template>

<style scoped>
.new-plan-page {
  width: min(100%, 960px);
  min-width: 0;
}

.back-button {
  margin-bottom: 24px;
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
  max-width: 960px;
}

.full-width-control {
  width: 100%;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  width: 100%;
  padding-top: 8px;
}
</style>
