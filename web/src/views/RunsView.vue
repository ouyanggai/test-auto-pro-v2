<script setup lang="ts">
import { NButton, NEmpty, NSelect, NSpin } from 'naive-ui'
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import { fetchPlanRuns, formatTime, RunApiError } from '../features/runs/api'
import type { RunSummary } from '../features/runs/api'

// RunsView 是运行列表：替换占位页，只提供进入详情的能力（分析视图属 F-021）。
const router = useRouter()
const plans = ref<Array<{ label: string; value: string }>>([])
const selectedPlanId = ref<string>('')
const runs = ref<RunSummary[]>([])
const loading = ref(false)
const errorText = ref('')

// loadPlans 拉取计划下拉项；只取一页足够选择最近使用的计划。
async function loadPlans(): Promise<void> {
  try {
    const response = await fetch('/api/plans?limit=50')
    const envelope = await response.json() as { success?: boolean; data?: { items?: Array<{ id: number; name: string }> } }
    const items = envelope.data?.items || []
    plans.value = items.map((item) => ({ label: item.name, value: String(item.id) }))
    if (items.length > 0 && !selectedPlanId.value) {
      selectedPlanId.value = String(items[0].id)
      await loadRuns()
    }
  } catch {
    errorText.value = '暂时无法读取计划列表，请重试'
  }
}

// loadRuns 拉取所选计划的运行列表。
async function loadRuns(): Promise<void> {
  if (!selectedPlanId.value) return
  loading.value = true
  errorText.value = ''
  try {
    runs.value = await fetchPlanRuns(selectedPlanId.value)
  } catch (error) {
    errorText.value = error instanceof RunApiError ? error.message : '暂时无法读取运行列表，请重试'
    runs.value = []
  } finally {
    loading.value = false
  }
}

// openDetail 进入路径运行详情页。
function openDetail(run: RunSummary): void {
  router.push(`/runs/${run.runId}`)
}

onMounted(() => { void loadPlans() })
onBeforeUnmount(() => { /* 本页无常驻定时器 */ })
</script>

<template>
  <section class="runs-view">
    <header class="runs-view__header">
      <h1>运行记录</h1>
      <p class="runs-view__hint">每次启动都会生成独立且不可覆盖的运行记录；这里只提供进入详情查看。</p>
    </header>

    <div class="runs-view__toolbar">
      <NSelect
        v-model:value="selectedPlanId"
        class="runs-view__plan-select"
        :options="plans"
        placeholder="选择计划"
        filterable
        @update:value="loadRuns"
      />
      <NButton type="primary" :disabled="!selectedPlanId" @click="loadRuns">刷新</NButton>
    </div>

    <p v-if="errorText" class="runs-view__error" role="alert">{{ errorText }}</p>

    <div v-if="loading" class="runs-view__loading"><NSpin size="small" /><span>正在读取运行列表……</span></div>

    <NEmpty
      v-else-if="runs.length === 0"
      description="该计划还没有运行记录；在计划路径页通过运行前检查后即可启动单步运行。"
    />

    <table v-else class="runs-view__table">
      <thead>
        <tr>
          <th>运行号</th>
          <th>模式</th>
          <th>运行状态</th>
          <th>路径状态</th>
          <th>路径结果</th>
          <th>开始时间</th>
          <th>结束时间</th>
          <th aria-label="操作" />
        </tr>
      </thead>
      <tbody>
        <tr v-for="run in runs" :key="run.runId">
          <td>#{{ run.runNo }}</td>
          <td>{{ run.modeName }}</td>
          <td>{{ run.statusName }}</td>
          <td>{{ run.pathRunStatusName }}</td>
          <td>{{ run.resultName || '—' }}</td>
          <td>{{ formatTime(run.startedAt) }}</td>
          <td>{{ formatTime(run.finishedAt) }}</td>
          <td><NButton size="small" @click="openDetail(run)">进入详情</NButton></td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<style scoped>
.runs-view {
  display: grid;
  gap: 16px;
  padding: 20px 24px;
}

.runs-view__header h1 {
  margin: 0;
  font-size: 20px;
}

.runs-view__hint {
  margin: 6px 0 0;
  opacity: 0.75;
}

.runs-view__toolbar {
  display: flex;
  gap: 10px;
  align-items: center;
}

.runs-view__plan-select { width: 280px; }
.runs-view__error { color: var(--error-color, #d03050); }
.runs-view__loading { display: flex; gap: 10px; align-items: center; opacity: 0.8; }

.runs-view__table {
  border: 1px solid rgba(128, 128, 128, 0.35);
  border-collapse: collapse;
}

.runs-view__table th,
.runs-view__table td {
  padding: 8px 12px;
  text-align: left;
  border-bottom: 1px solid rgba(128, 128, 128, 0.25);
}
</style>
