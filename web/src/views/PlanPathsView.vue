<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { NButton, NDescriptions, NDescriptionsItem, NEmpty, NSpin, NTag } from 'naive-ui'
import { useRoute, useRouter } from 'vue-router'

import { planStatusLabels } from '../features/plans/logic'
import { fetchPlan, PlanApiError } from '../features/plans/persistence'
import { flowSourceLabels } from '../features/plans/selection'
import type { PersistedPlan } from '../features/plans/types'

const route = useRoute()
const router = useRouter()
const plan = ref<PersistedPlan | null>(null)
const loading = ref(false)
const error = ref('')
const notFound = ref(false)
let loadController: AbortController | null = null
let loadVersion = 0

const planID = computed(() => String(route.params.id || ''))

async function loadPlan() {
  loadController?.abort()
  const controller = new AbortController()
  loadController = controller
  const version = ++loadVersion
  loading.value = true
  error.value = ''
  notFound.value = false
  plan.value = null
  try {
    const result = await fetchPlan(planID.value, controller.signal)
    if (controller.signal.aborted || version !== loadVersion) return
    plan.value = result
  }
  catch (caught) {
    if (controller.signal.aborted || version !== loadVersion) return
    const apiError = caught instanceof PlanApiError ? caught : new PlanApiError('暂时无法读取计划，请重试')
    notFound.value = apiError.code === 'PLAN_NOT_FOUND'
    error.value = apiError.message
  }
  finally {
    if (version === loadVersion) loading.value = false
  }
}

function runModeText(value: PersistedPlan): string {
  if (value.runMode === 'parallel') return `并行（最大 ${value.maxConcurrency ?? '-'}）`
  return '串行'
}

function scheduledAtText(value: string | null): string {
  if (!value) return '未设置'
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? '时间异常' : parsed.toLocaleString('zh-CN', { hour12: false })
}

watch(planID, () => { void loadPlan() }, { immediate: true })
onBeforeUnmount(() => loadController?.abort())
</script>

<template>
  <section class="plan-paths-page">
    <div class="paths-back-bar">
      <n-button text type="primary" @click="router.push('/plans')">返回测试计划</n-button>
    </div>

    <n-spin :show="loading">
      <div v-if="plan" class="paths-content">
        <header class="page-heading">
          <div>
            <h1>{{ plan.name }}</h1>
            <p>计划已经保存，可以稍后继续配置。</p>
          </div>
          <n-tag size="small" type="warning" :bordered="false">
            {{ planStatusLabels[plan.status] }}
          </n-tag>
        </header>

        <n-descriptions label-placement="left" :column="2" bordered>
          <n-descriptions-item label="目标流程">{{ plan.targetObjectName }}</n-descriptions-item>
          <n-descriptions-item label="发起账号">
            {{ plan.accountDisplayName ? `${plan.accountDisplayName}（${plan.account}）` : plan.account }}
          </n-descriptions-item>
          <n-descriptions-item label="流程来源">{{ flowSourceLabels[plan.flowSource] }}</n-descriptions-item>
          <n-descriptions-item label="运行方式">{{ runModeText(plan) }}</n-descriptions-item>
          <n-descriptions-item label="定时启动">{{ scheduledAtText(plan.scheduledAt) }}</n-descriptions-item>
          <n-descriptions-item label="路径数量">{{ plan.pathCount }}</n-descriptions-item>
        </n-descriptions>

        <div class="path-empty-region">
          <n-empty description="还没有选择执行路径，计划已经保存，可以稍后继续">
            <template #extra>
              <span class="path-boundary-copy">真实流程结构与路径选择将在后续功能开放。</span>
            </template>
          </n-empty>
        </div>
      </div>

      <div v-else-if="!loading" class="paths-error-region">
        <n-empty :description="notFound ? '计划不存在或已不可用' : error || '暂时无法读取计划'">
          <template #extra>
            <div class="error-actions">
              <n-button v-if="!notFound" type="primary" secondary @click="loadPlan">重试</n-button>
              <n-button @click="router.push('/plans')">返回测试计划</n-button>
            </div>
          </template>
        </n-empty>
      </div>
    </n-spin>
  </section>
</template>

<style scoped>
.plan-paths-page {
  width: 100%;
  min-width: 0;
}

.paths-back-bar {
  position: sticky;
  top: -32px;
  z-index: 5;
  margin: -32px -48px 24px;
  padding: 16px 48px 12px;
  background: var(--n-color);
  border-bottom: 1px solid var(--n-border-color);
}

.paths-content {
  width: min(960px, 100%);
  margin: 0 auto;
}

.page-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 28px;
}

.page-heading h1 {
  margin: 0 0 8px;
  font-size: 28px;
  font-weight: 600;
}

.page-heading p,
.path-boundary-copy {
  color: var(--n-text-color-2);
}

.page-heading p {
  margin: 0;
}

.path-empty-region,
.paths-error-region {
  display: grid;
  min-height: 320px;
  place-items: center;
  border-top: 1px solid var(--n-border-color);
  margin-top: 32px;
}

.error-actions {
  display: flex;
  gap: 12px;
}
</style>
