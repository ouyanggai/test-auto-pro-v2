<script setup lang="ts">
import { NButton, NCheckbox, NPagination, NPopconfirm, NSelect, NSpin } from 'naive-ui'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import AppEmptyState from '../components/AppEmptyState.vue'
import { deleteRun, fetchAllRunsPaged, formatTime, RunApiError } from '../features/runs/api'
import type { RunSummary } from '../features/runs/api'

// RunsView 是运行记录列表：一行只对应一次计划运行（跨计划）。
// 每行显示计划、运行方式、整体状态、路径汇总、开始/结束时间，并提供进入与删除入口（2026-09-06）。
const router = useRouter()
const runs = ref<RunSummary[]>([])
const loading = ref(false)
const errorText = ref('')

// 分页：页码从 1 起，总数由后端返回；切页或改每页大小都重新拉取。
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

// runStatusFilter 是运行状态筛选：空串表示全部；筛选在服务端完成，空结果与读取失败语义分开。
const runStatusFilter = ref('')
const runStatusOptions = [
  { label: '全部状态', value: '' },
  { label: '运行中', value: 'running' },
  { label: '已完成', value: 'completed' },
  { label: '失败', value: 'failed' },
  { label: '已停止', value: 'stopped' },
  { label: '已取消', value: 'cancelled' },
]

// loadRuns 拉取运行列表（可按状态筛选、分页）。
async function loadRuns(): Promise<void> {
  loading.value = true
  errorText.value = ''
  try {
    const result = await fetchAllRunsPaged(runStatusFilter.value, page.value, pageSize.value)
    runs.value = result.items
    total.value = result.total
    // 删除或筛选后当前页可能越界（如末页删空）：回退到最后一页重新拉取。
    const maxPage = Math.max(1, Math.ceil(result.total / pageSize.value))
    if (page.value > maxPage) {
      page.value = maxPage
      const retry = await fetchAllRunsPaged(runStatusFilter.value, page.value, pageSize.value)
      runs.value = retry.items
      total.value = retry.total
    }
  } catch (error) {
    errorText.value = error instanceof RunApiError ? error.message : '暂时无法读取任务列表，请重试'
    runs.value = []
  } finally {
    loading.value = false
  }
}

// changePage 切换页码后重新拉取。
function changePage(next: number): void {
  page.value = next
  void loadRuns()
}

// changePageSize 改每页条数后回到第一页重新拉取。
function changePageSize(size: number): void {
  pageSize.value = size
  page.value = 1
  void loadRuns()
}

// openPaths 进入二级路径页：本次运行的执行路径列表。
function openPaths(run: RunSummary): void {
  router.push(`/runs/${run.runId}`)
}

// deletingRunId 是正在删除的运行：删除请求在途时按钮进入忙碌态，重复点击不会发出第二个请求。
const deletingRunId = ref<number>(0)

// checkedRunIds 是批量删除的勾选集合；逐行复选框 + 表头全选。
const checkedRunIds = ref<Set<number>>(new Set())
const checkedCount = computed(() => checkedRunIds.value.size)
const allChecked = computed(() => runs.value.length > 0 && runs.value.every(run => checkedRunIds.value.has(run.runId)))
// deletingBatch 标记批量删除请求在途。
const deletingBatch = ref(false)

// toggleChecked 维护单行勾选；加载后勾选集合会被重置，避免残留已不在列表里的 ID。
function toggleChecked(runId: number, checked: boolean): void {
  const next = new Set(checkedRunIds.value)
  if (checked) next.add(runId)
  else next.delete(runId)
  checkedRunIds.value = next
}

// toggleAllChecked 表头全选/取消全选。
function toggleAllChecked(checked: boolean): void {
  checkedRunIds.value = checked ? new Set(runs.value.map(run => run.runId)) : new Set()
}

// removeChecked 批量删除勾选的运行：逐条调用单删接口，失败的跳过并在最后汇总，
// 不因为其中一条失败而中断其余删除（用户意图是清掉这批记录）。
async function removeChecked(): Promise<void> {
  if (deletingBatch.value || checkedCount.value === 0) return
  deletingBatch.value = true
  errorText.value = ''
  const failures: string[] = []
  try {
    for (const runId of checkedRunIds.value) {
      try {
        await deleteRun(String(runId))
      } catch (error) {
        failures.push(`运行 ${runId}：${error instanceof RunApiError ? error.message : '删除失败'}`)
      }
    }
    if (failures.length) {
      errorText.value = `部分删除失败：${failures.join('；')}`
    }
    checkedRunIds.value = new Set()
    await loadRuns()
  } finally {
    deletingBatch.value = false
  }
}

// batchDeleteHint 写清批量删除的影响范围与数量。
const batchDeleteHint = computed(() =>
  `删除勾选的 ${checkedCount.value} 次运行：各次运行的全部路径记录、步骤与日志引用会一并删除；目标平台上的实例和业务数据不受影响。删除后无法恢复。`)

// removeRun 删除整次工具侧运行；只删除工具侧记录，目标平台实例与业务数据不受影响。
async function removeRun(run: RunSummary): Promise<void> {
  if (deletingRunId.value !== 0) return
  deletingRunId.value = run.runId
  errorText.value = ''
  try {
    await deleteRun(String(run.runId))
    await loadRuns()
  } catch (error) {
    errorText.value = error instanceof RunApiError ? error.message : '删除失败，请重试'
  } finally {
    deletingRunId.value = 0
  }
}

// deleteHint 写清删除的影响范围：删除什么、不删除什么。
function deleteHint(run: RunSummary): string {
  const pathCount = run.pathRunCount ?? 1
  return `删除计划「${run.planName || '未知计划'}」的运行 #${run.runNo}：共 ${pathCount} 条执行路径的记录、步骤与日志引用会一并删除；目标平台上的实例和业务数据不受影响。删除后无法恢复。`
}

// changeStatusFilter 状态筛选变化后回到第一页重新拉取。
function changeStatusFilter(): void {
  page.value = 1
  void loadRuns()
}

// clearRunStatusFilter 清除状态筛选并回到第一页重新读取完整运行列表。
function clearRunStatusFilter(): void {
  runStatusFilter.value = ''
  page.value = 1
  void loadRuns()
}

onMounted(() => { void loadRuns() })
onBeforeUnmount(() => { /* 本页无常驻定时器 */ })
</script>

<template>
  <section class="runs-view">
    <header class="runs-view__header">
      <h1>任务列表</h1>
      <p class="runs-view__hint">每次启动都会生成一条独立任务；打开任务可以查看本次执行的各条路径。</p>
    </header>

    <div class="runs-view__toolbar">
      <NSelect
        v-model:value="runStatusFilter"
        class="runs-view__status-select"
        :options="runStatusOptions"
        placeholder="全部状态"
        @update:value="changeStatusFilter"
      />
      <NButton class="app-button--solid-primary" type="primary" @click="loadRuns">刷新</NButton>
      <NPopconfirm v-if="checkedCount > 0" @positive-click="removeChecked">
        <template #trigger>
          <NButton type="error" ghost :loading="deletingBatch">删除选中（{{ checkedCount }}）</NButton>
        </template>
        {{ batchDeleteHint }}
      </NPopconfirm>
    </div>

    <p v-if="errorText" class="runs-view__error" role="alert">{{ errorText }}</p>

    <div v-if="loading" class="runs-view__loading"><NSpin size="small" /><span>正在读取任务列表……</span></div>

    <AppEmptyState
      v-else-if="runs.length === 0 && !errorText"
      size="large"
      :title="runStatusFilter ? '没有符合条件的任务' : '还没有任务'"
      :description="runStatusFilter
        ? '当前筛选条件下没有结果，清除筛选后可以查看全部任务。'
        : '运行测试计划后，每次运行的结果都会按时间出现在这里。'"
    >
      <NButton v-if="runStatusFilter" secondary type="primary" @click="clearRunStatusFilter">查看全部任务</NButton>
      <NButton class="app-button--solid-primary" v-else type="primary" @click="router.push('/plans')">前往测试计划</NButton>
    </AppEmptyState>

    <table v-else class="runs-view__table">
      <thead>
        <tr>
          <th class="runs-view__check-col">
            <NCheckbox
              :checked="allChecked"
              :indeterminate="checkedCount > 0 && !allChecked"
              @update:checked="toggleAllChecked"
            />
          </th>
          <th>计划</th>
          <th>运行号</th>
          <th>运行方式</th>
          <th>整体状态</th>
          <th>路径情况</th>
          <th>开始时间</th>
          <th>结束时间</th>
          <th aria-label="操作" />
        </tr>
      </thead>
      <tbody>
        <tr v-for="run in runs" :key="run.runId">
          <td class="runs-view__check-col">
            <NCheckbox
              :checked="checkedRunIds.has(run.runId)"
              @update:checked="value => toggleChecked(run.runId, Boolean(value))"
            />
          </td>
          <td class="runs-view__plan">{{ run.planName || `计划 ${run.planId}` }}</td>
          <td>#{{ run.runNo }}</td>
          <td>{{ run.modeName }}</td>
          <td>{{ run.statusName }}<template v-if="run.resultName"> · {{ run.resultName }}</template></td>
          <td>
            <template v-if="(run.pathRunCount ?? 0) > 1">{{ run.pathsSummary }}</template>
            <template v-else>{{ run.pathRunStatusName }}</template>
          </td>
          <td>{{ formatTime(run.startedAt) }}</td>
          <td>{{ formatTime(run.finishedAt) }}</td>
          <td class="runs-view__actions">
            <NButton size="small" @click="openPaths(run)">查看路径</NButton>
            <NPopconfirm @positive-click="removeRun(run)">
              <template #trigger>
                <NButton
                  size="small"
                  type="error"
                  ghost
                  :loading="deletingRunId === run.runId"
                  :disabled="deletingRunId !== 0"
                >删除</NButton>
              </template>
              {{ deleteHint(run) }}
            </NPopconfirm>
          </td>
        </tr>
      </tbody>
    </table>

    <div v-if="!loading && total > 0" class="runs-view__pagination">
      <NPagination
        :page="page"
        :page-size="pageSize"
        :item-count="total"
        :page-sizes="[10, 20, 50, 100]"
        show-size-picker
        show-quick-jumper
        @update:page="changePage"
        @update:page-size="changePageSize"
      >
        <template #prefix="{ itemCount }">共 {{ itemCount }} 条</template>
      </NPagination>
    </div>
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

.runs-view__status-select { width: 160px; }
.runs-view__check-col { width: 40px; text-align: center; }
.runs-view__error { color: var(--error-color); }
.runs-view__loading { display: flex; gap: 10px; align-items: center; opacity: 0.8; }

.runs-view__table {
  border: 1px solid var(--divider-color);
  border-collapse: collapse;
}

.runs-view__table th,
.runs-view__table td {
  padding: 8px 12px;
  text-align: left;
  border-bottom: 1px solid var(--divider-color);
}

.runs-view__plan { font-weight: 500; }
.runs-view__actions { display: flex; gap: 8px; white-space: nowrap; }
.runs-view__pagination { display: flex; align-items: center; justify-content: flex-end; }
</style>
