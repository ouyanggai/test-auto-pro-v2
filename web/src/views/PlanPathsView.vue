<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { NAlert, NButton, NDescriptions, NDescriptionsItem, NEmpty, NSpin, NTag } from 'naive-ui'
import { useRoute, useRouter } from 'vue-router'

import FlowGraphCanvas from '../features/flow-graph/FlowGraphCanvas.vue'
import { fetchFlowGraph, FlowGraphApiError } from '../features/flow-graph/api'
import type { FlowGraph } from '../features/flow-graph/types'
import { planStatusLabels } from '../features/plans/logic'
import { fetchPlan, PlanApiError } from '../features/plans/persistence'
import { flowSourceLabels } from '../features/plans/selection'
import type { PersistedPlan } from '../features/plans/types'

const route = useRoute()
const router = useRouter()
const plan = ref<PersistedPlan | null>(null)
const graph = ref<FlowGraph | null>(null)
const planLoading = ref(false)
const graphLoading = ref(false)
const planError = ref('')
const graphError = ref<FlowGraphApiError | null>(null)
const planNotFound = ref(false)
let loadController: AbortController | null = null
let loadVersion = 0

const planID = computed(() => String(route.params.id || ''))

async function loadPage() {
  loadController?.abort()
  const controller = new AbortController()
  loadController = controller
  const version = ++loadVersion
  planLoading.value = true
  graphLoading.value = false
  planError.value = ''
  graphError.value = null
  planNotFound.value = false
  plan.value = null
  graph.value = null
  try {
    const storedPlan = await fetchPlan(planID.value, controller.signal)
    if (controller.signal.aborted || version !== loadVersion) return
    plan.value = storedPlan
    planLoading.value = false
    graphLoading.value = true
    try {
      const currentGraph = await fetchFlowGraph(planID.value, controller.signal)
      if (controller.signal.aborted || version !== loadVersion) return
      graph.value = currentGraph
    }
    catch (caught) {
      if (controller.signal.aborted || version !== loadVersion) return
      graphError.value = caught instanceof FlowGraphApiError
        ? caught
        : new FlowGraphApiError('暂时无法读取流程，请重试', { retryable: true })
    }
    finally {
      if (version === loadVersion) graphLoading.value = false
    }
  }
  catch (caught) {
    if (controller.signal.aborted || version !== loadVersion) return
    const apiError = caught instanceof PlanApiError ? caught : new PlanApiError('暂时无法读取计划，请重试')
    planNotFound.value = apiError.code === 'PLAN_NOT_FOUND'
    planError.value = apiError.message
  }
  finally {
    if (version === loadVersion) planLoading.value = false
  }
}

async function retryGraph() {
  if (!plan.value) {
    await loadPage()
    return
  }
  loadController?.abort()
  const controller = new AbortController()
  loadController = controller
  const version = ++loadVersion
  graphLoading.value = true
  graphError.value = null
  graph.value = null
  try {
    graph.value = await fetchFlowGraph(planID.value, controller.signal)
  }
  catch (caught) {
    if (controller.signal.aborted || version !== loadVersion) return
    graphError.value = caught instanceof FlowGraphApiError
      ? caught
      : new FlowGraphApiError('暂时无法读取流程，请重试', { retryable: true })
  }
  finally {
    if (version === loadVersion) graphLoading.value = false
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

watch(planID, () => { void loadPage() }, { immediate: true })
onBeforeUnmount(() => loadController?.abort())
</script>

<template>
  <section class="plan-paths-page">
    <div class="paths-back-bar">
      <n-button text type="primary" @click="router.push('/plans')">返回测试计划</n-button>
    </div>

    <n-spin :show="planLoading">
      <div v-if="plan" class="paths-content">
        <header class="page-heading">
          <div>
            <h1>{{ plan.name }}</h1>
            <p>查看当前流程结构，路径选择将在下一步开放。</p>
          </div>
          <n-tag size="small" type="warning" :bordered="false">
            {{ planStatusLabels[plan.status] }}
          </n-tag>
        </header>

        <n-descriptions label-placement="left" :column="3" bordered size="small">
          <n-descriptions-item label="目标流程">{{ plan.targetObjectName }}</n-descriptions-item>
          <n-descriptions-item label="发起账号">
            {{ plan.accountDisplayName ? `${plan.accountDisplayName}（${plan.account}）` : plan.account }}
          </n-descriptions-item>
          <n-descriptions-item label="流程来源">{{ flowSourceLabels[plan.flowSource] }}</n-descriptions-item>
          <n-descriptions-item label="运行方式">{{ runModeText(plan) }}</n-descriptions-item>
          <n-descriptions-item label="定时启动">{{ scheduledAtText(plan.scheduledAt) }}</n-descriptions-item>
          <n-descriptions-item label="路径数量">{{ plan.pathCount }}</n-descriptions-item>
        </n-descriptions>

        <section class="graph-section" aria-labelledby="flow-graph-heading">
          <div class="graph-heading">
            <div>
              <h2 id="flow-graph-heading">流程结构</h2>
              <p>可拖动画布、缩放或适配视口，节点和连线均为只读。</p>
            </div>
          </div>

          <div class="graph-region">
            <div v-if="graphLoading" class="graph-state">
              <n-spin size="large" description="正在读取流程结构" />
            </div>
            <template v-else-if="graph">
              <n-alert v-if="graph.warnings.length" class="graph-warning" type="warning" :show-icon="false">
                {{ graph.warnings.join('；') }}
              </n-alert>
              <flow-graph-canvas :graph="graph" />
            </template>
            <div v-else class="graph-state">
              <n-empty :description="graphError?.message || '暂时无法读取流程'">
                <template #extra>
                  <n-button type="primary" secondary @click="retryGraph">重试</n-button>
                </template>
              </n-empty>
            </div>
          </div>
        </section>
      </div>

      <div v-else-if="!planLoading" class="paths-error-region">
        <n-empty :description="planNotFound ? '计划不存在或已不可用' : planError || '暂时无法读取计划'">
          <template #extra>
            <div class="error-actions">
              <n-button v-if="!planNotFound" type="primary" secondary @click="loadPage">重试</n-button>
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
  width: min(1180px, 100%);
  margin: 0 auto;
}

.page-heading,
.graph-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
}

.page-heading {
  margin-bottom: 24px;
}

.page-heading h1,
.graph-heading h2 {
  margin: 0;
  font-weight: 600;
}

.page-heading h1 {
  margin-bottom: 8px;
  font-size: 28px;
}

.graph-heading h2 {
  margin-bottom: 5px;
  font-size: 18px;
}

.page-heading p,
.graph-heading p {
  margin: 0;
  color: var(--n-text-color-2);
}

.graph-section {
  margin-top: 28px;
}

.graph-heading {
  margin-bottom: 14px;
}

.graph-region,
.graph-state,
.paths-error-region {
  min-height: 560px;
}

.graph-region {
  position: relative;
  min-width: 0;
}

.graph-state,
.paths-error-region {
  display: grid;
  place-items: center;
  border-top: 1px solid var(--n-border-color);
}

.graph-warning {
  position: absolute;
  top: 12px;
  right: 16px;
  left: 16px;
  z-index: 4;
}

.error-actions {
  display: flex;
  gap: 12px;
}
</style>
