<script setup lang="ts">
import { computed, h, onBeforeUnmount, reactive, ref, watch } from 'vue'
import {
  NButton,
  NDataTable,
	NInput,
	NPopconfirm,
  NSelect,
  NTag,
  useMessage,
  type DataTableColumns,
  type DataTableRowKey,
  type SelectOption,
} from 'naive-ui'
import { useRouter } from 'vue-router'

import AppEmptyState from '../components/AppEmptyState.vue'
import { fetchExecutionPaths } from '../features/execution-paths/api'
import { planStatusLabels, planStatusOptions } from '../features/plans/logic'
import { batchDeletePlans, deletePlan, fetchPlansPaged, PlanApiError } from '../features/plans/persistence'
import type { PlanFilters, PlanRow, PlanStatus } from '../features/plans/types'
import RunPreflightDialog from '../features/run-readiness/RunPreflightDialog.vue'

const message = useMessage()
const router = useRouter()
const filters = reactive<PlanFilters>({ name: '', status: null })
const plans = ref<PlanRow[]>([])
const loading = ref(false)
const loadError = ref('')
let loadVersion = 0
let loadController: AbortController | null = null

// 批量操作：选中的计划 ID 集合
const checkedRowKeys = ref<DataTableRowKey[]>([])
const batchDeleting = ref(false)

// canBatchDelete 只允许批量删除「非运行中」状态的计划（未运行/已完成/失败都可删除）
const canBatchDelete = computed(() => {
  if (checkedRowKeys.value.length === 0) return false
  return checkedRowKeys.value.every(id => {
    const plan = plans.value.find(p => p.id === id)
    return plan?.status !== 'running'
  })
})

const selectedCount = computed(() => checkedRowKeys.value.length)

// 服务端分页：页码从 1 起，总数由后端返回。
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const pagination = computed(() => ({
  page: page.value,
  pageSize: pageSize.value,
  itemCount: total.value,
  showSizePicker: true,
  showQuickJumper: true,
  pageSizes: [10, 20, 50, 100],
  prefix: () => `共 ${total.value} 条`,
}))

// changePage 切页后重新拉取。
function changePage(next: number): void {
  page.value = next
  void loadPlans()
}

// changePageSize 改每页条数后回到第一页。
function changePageSize(size: number): void {
  pageSize.value = size
  page.value = 1
  void loadPlans()
}

const statusOptions: SelectOption[] = planStatusOptions
const hasFilters = computed(() => Boolean(filters.name || filters.status))

function clearFilters() {
  filters.name = ''
  filters.status = null
}

function editPlan(plan: PlanRow) { void router.push(`/plans/${plan.id}/edit`) }
function editPaths(plan: PlanRow) { void router.push(`/plans/${plan.id}/paths`) }

// runFromList 从计划列表直接发起一次运行：默认勾选「已配置且数据就绪」的路径，
// 与路径页此前的默认口径一致；运行前检查弹窗只检查这些路径。
const runPreflightOpen = ref(false)
const runTarget = ref<{ planId: string; planName: string; pathIds: string[] } | null>(null)
const runPreparing = ref<string>('')

async function runFromList(plan: PlanRow): Promise<void> {
  runPreparing.value = plan.id
  try {
    const paths = await fetchExecutionPaths(plan.id, new AbortController().signal)
    const selected = paths
      .filter(path => path.configurationStatus === 'configured' && path.dataStatus === 'ready')
      .map(path => path.id)
    if (selected.length === 0) {
      message.warning('该计划还没有可执行的路径：请先完成节点配置与基础表单数据')
      return
    }
    runTarget.value = { planId: plan.id, planName: plan.name, pathIds: selected }
    runPreflightOpen.value = true
  } catch (error) {
    message.error(error instanceof Error ? error.message : '暂时无法读取执行路径，请重试')
  } finally {
    runPreparing.value = ''
  }
}

// locateReadinessItem 把运行前检查里的阻塞项定位到那条路径的对应面板。
function locateReadinessItem(pathId: string, anchor: string) {
  if (!runTarget.value) return
  const query = anchor ? '?panel=' + encodeURIComponent(anchor) : ''
  void router.push('/plans/' + runTarget.value.planId + '/paths/' + pathId + '/configure' + query)
}

// removePlan 删除本系统开发计划后重读列表；当前目标平台不会收到任何写请求。
async function removePlan(plan: PlanRow) {
	try {
		await deletePlan(plan.id)
		plans.value = plans.value.filter(item => item.id !== plan.id)
		await loadPlans()
	}
	catch (error) {
		const apiError = error instanceof PlanApiError ? error : new PlanApiError('删除计划失败，请重试')
		loadError.value = apiError.message
	}
}

// batchRemovePlans 批量删除选中计划；只有运行中的计划不能删除。
async function batchRemovePlans() {
  if (!canBatchDelete.value) return
  batchDeleting.value = true
  try {
    const ids = checkedRowKeys.value.map(String)
    const result = await batchDeletePlans(ids)
    if (result.failures.length > 0) {
      const failureMsg = result.failures.map(f => `计划 ${f.id}: ${f.message}`).join('；')
      message.warning(`成功删除 ${result.successCount} 个计划，${result.failures.length} 个失败：${failureMsg}`)
    } else {
      message.success(`已删除 ${result.successCount} 个计划`)
    }
    checkedRowKeys.value = []
    await loadPlans()
  }
  catch (error) {
    const apiError = error instanceof PlanApiError ? error : new PlanApiError('批量删除失败，请重试')
    message.error(apiError.message)
  }
  finally {
    batchDeleting.value = false
  }
}

async function loadPlans() {
  loadController?.abort()
  const controller = new AbortController()
  loadController = controller
  const version = ++loadVersion
  loading.value = true
  loadError.value = ''
  try {
    const result = await fetchPlansPaged(filters, page.value, pageSize.value, controller.signal)
    if (version !== loadVersion || controller.signal.aborted) return
    plans.value = result.items
    total.value = result.total
    // 删除或筛选后当前页可能越界（末页删空）：回退到最后一页重新拉取。
    const maxPage = Math.max(1, Math.ceil(result.total / pageSize.value))
    if (page.value > maxPage) {
      page.value = maxPage
      const retry = await fetchPlansPaged(filters, page.value, pageSize.value, controller.signal)
      if (version !== loadVersion || controller.signal.aborted) return
      plans.value = retry.items
      total.value = retry.total
    }
  }
  catch (error) {
    if (controller.signal.aborted || version !== loadVersion) return
    const apiError = error instanceof PlanApiError ? error : new PlanApiError('暂时无法读取计划，请重试')
    loadError.value = apiError.message
    plans.value = []
  }
  finally {
    if (version === loadVersion) loading.value = false
  }
}

function formatScheduledAt(value: string | null): string {
  if (!value) return '未设置'
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? '时间异常' : parsed.toLocaleString('zh-CN', { hour12: false })
}

watch([() => filters.name, () => filters.status], () => { page.value = 1; void loadPlans() }, { immediate: true })
onBeforeUnmount(() => loadController?.abort())

function statusTagType(status: PlanStatus): 'default' | 'success' | 'warning' | 'info' {
  if (status === 'running') return 'warning'
  if (status === 'completed') return 'info'
  return 'default'
}

const columns: DataTableColumns<PlanRow> = [
  { type: 'selection' },
  { title: '计划名称', key: 'name', width: 220, ellipsis: { tooltip: true } },
  { title: '流程名称', key: 'flowName', width: 180, ellipsis: { tooltip: true } },
  { title: '发起账号', key: 'accountName', width: 190, ellipsis: { tooltip: true } },
  { title: '路径数量', key: 'pathCount', width: 100, align: 'center' },
  {
    title: '运行方式',
    key: 'runMode',
    width: 100,
    render: (row) => (row.runMode === 'serial' ? '串行' : '并行'),
  },
  {
    title: '定时时间',
    key: 'scheduledAt',
    width: 170,
    render: (row) => formatScheduledAt(row.scheduledAt),
  },
  {
    title: '计划状态',
    key: 'status',
    width: 110,
    render: (row) => h(NTag, { size: 'small', type: statusTagType(row.status), bordered: false }, { default: () => planStatusLabels[row.status] }),
  },
  { title: '最近任务结果', key: 'lastRunResult', width: 180, ellipsis: { tooltip: true } },
  {
    title: '操作',
    key: 'actions',
    width: 340,
    fixed: 'right',
    render: (row) => {
    return h('div', {
        class: 'plan-row-actions',
        style: { display: 'flex', alignItems: 'center', gap: '12px', flexWrap: 'wrap' },
      }, [
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            'data-testid': 'plan-run-button',
            loading: runPreparing.value === row.id,
            onClick: () => void runFromList(row),
          },
          { default: () => '运行' },
        ),
        h(NButton, { size: 'small', secondary: true, type: 'primary', onClick: () => editPlan(row) }, { default: () => '编辑计划' }),
        h(NButton, { size: 'small', secondary: true, onClick: () => editPaths(row) }, { default: () => '编辑路径' }),
        row.status !== 'running' ? h('span', { class: 'plan-row-actions__delete' }, [
          h(NPopconfirm, { positiveText: '删除计划', negativeText: '取消', onPositiveClick: () => void removePlan(row) }, {
            default: () => '删除后会清除本系统中的路径和配置，不能恢复。',
            trigger: () => h(NButton, { size: 'small', secondary: true, type: 'error' }, { default: () => '删除' }),
          }),
        ]) : null,
      ])
    },
  },
]
</script>

<template>
  <section class="plan-page">
    <header class="page-heading">
      <div>
        <h1>测试计划</h1>
        <p>查看并筛选已经保存的测试计划。</p>
      </div>
      <n-button type="primary" @click="router.push('/plans/new')">新建计划</n-button>
    </header>

    <div class="plan-filters" aria-label="测试计划筛选">
      <n-input
        v-model:value="filters.name"
        class="plan-name-filter"
        clearable
        placeholder="搜索计划名称"
      />
      <n-select
        v-model:value="filters.status"
        class="plan-status-filter"
        clearable
        :options="statusOptions"
        placeholder="全部状态"
      />
      <n-button :disabled="!hasFilters" @click="clearFilters">清空</n-button>
      <div v-if="selectedCount > 0" class="batch-actions">
        <span class="batch-count">已选 {{ selectedCount }} 项</span>
        <n-popconfirm
          :positive-text="`删除 ${selectedCount} 个计划`"
          negative-text="取消"
          :disabled="!canBatchDelete"
          @positive-click="batchRemovePlans"
        >
          <template #trigger>
            <n-button
              secondary
              type="error"
              size="small"
              :disabled="!canBatchDelete"
              :loading="batchDeleting"
            >
              批量删除
            </n-button>
          </template>
          删除后会清除本系统中的路径和配置，不能恢复。{{ canBatchDelete ? '' : '运行中的计划不能删除。' }}
        </n-popconfirm>
      </div>
    </div>

    <div v-if="loadError" class="plan-load-error" role="alert">
      <span>{{ loadError }}</span>
      <n-button text type="primary" @click="loadPlans">重试</n-button>
    </div>

    <div v-else class="plan-table-region">
      <n-data-table
        remote
        :columns="columns"
        :data="plans"
        :loading="loading"
        :row-key="(row: PlanRow) => row.id"
        :checked-row-keys="checkedRowKeys"
        :scroll-x="1538"
        :single-line="false"
        :pagination="pagination"
        striped
        @update:checked-row-keys="(keys: DataTableRowKey[]) => checkedRowKeys = keys"
        @update:page="changePage"
        @update:page-size="changePageSize"
      >
        <template #empty>
          <app-empty-state
            v-if="!loading"
            :title="hasFilters ? '没有符合条件的测试计划' : '还没有测试计划'"
            :description="hasFilters
              ? '当前筛选条件下没有结果，清除筛选后可以查看全部计划。'
              : '新建一个测试计划，开始配置需要验证的流程和执行路径。'"
          >
            <n-button v-if="hasFilters" secondary type="primary" @click="clearFilters">查看全部计划</n-button>
            <n-button v-else type="primary" @click="router.push('/plans/new')">新建计划</n-button>
          </app-empty-state>
        </template>
      </n-data-table>
    </div>
  </section>
  <run-preflight-dialog
    :show="runPreflightOpen"
    :plan-id="runTarget?.planId ?? ''"
    :path-ids="runTarget?.pathIds ?? []"
    @update:show="value => (runPreflightOpen = value)"
    @locate="locateReadinessItem"
  />
</template>

<style scoped>
.plan-page {
  width: 100%;
  min-width: 0;
}

.page-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 32px;
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
  color: var(--text-color-2);
  line-height: 1.7;
}

.plan-filters {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
  flex-wrap: wrap;
}

.plan-name-filter {
  width: 280px;
}

.plan-status-filter {
  width: 180px;
}

.batch-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-left: auto;
  padding: 8px 16px;
  background: var(--ta-reading);
  border-radius: 6px;
}

.batch-count {
  font-size: 14px;
  color: var(--text-color-2);
}

.plan-table-region {
  width: 100%;
  min-width: 0;
  overflow: hidden;
}

.plan-load-error {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 120px;
  color: var(--text-color-2);
}

:deep(.plan-row-actions) {
  display: flex;
  align-items: center;
  width: 100%;
  min-width: 0;
  gap: 12px;
  white-space: nowrap;
}

:deep(.plan-row-actions > *) {
  flex: 0 0 auto;
}

:deep(.plan-row-actions__delete) {
  display: flex;
}

@media (max-width: 680px) {
  :deep(.plan-row-actions) {
    flex-wrap: wrap;
    row-gap: 6px;
  }
}
</style>
