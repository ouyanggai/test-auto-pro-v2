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
  type SelectOption,
} from 'naive-ui'
import { useRouter } from 'vue-router'

import AppEmptyState from '../components/AppEmptyState.vue'
import { fetchExecutionPaths } from '../features/execution-paths/api'
import { getPlanAction, planStatusLabels, planStatusOptions } from '../features/plans/logic'
import { deletePlan, fetchPlans, PlanApiError } from '../features/plans/persistence'
import type { PlanFilters, PlanRow, PlanStatus } from '../features/plans/types'
import RunPreflightDialog from '../features/run-readiness/RunPreflightDialog.vue'

const message = useMessage()
const router = useRouter()
const filters = reactive<PlanFilters>({ name: '', status: null })
const prototypeNotice = ref('')
const plans = ref<PlanRow[]>([])
const loading = ref(false)
const loadError = ref('')
let loadVersion = 0
let loadController: AbortController | null = null

const statusOptions: SelectOption[] = planStatusOptions
const hasFilters = computed(() => Boolean(filters.name || filters.status))

function clearFilters() {
  filters.name = ''
  filters.status = null
}

function handlePlanAction(plan: PlanRow) {
  const action = getPlanAction(plan.status)
  if (action.intent === 'configure') {
    void router.push(`/plans/${plan.id}/paths`)
    return
  }
  prototypeNotice.value = `“${plan.name}”的“${action.label}”当前仅用于静态原型展示，真实业务将在后续功能接入。`
}

// runFromList 从计划列表直接发起一次运行：默认勾选「已配置且数据就绪」的路径，
// 与路径页此前的默认口径一致；运行前检查弹窗只检查这些路径。
const runPreflightOpen = ref(false)
const runTarget = ref<{ planId: string; planName: string; pathIds: string[] } | null>(null)
const runPreparing = ref<string>('')

async function runFromList(plan: PlanRow): Promise<void> {
  runPreparing.value = plan.id
  prototypeNotice.value = ''
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
	prototypeNotice.value = ''
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

async function loadPlans() {
  loadController?.abort()
  const controller = new AbortController()
  loadController = controller
  const version = ++loadVersion
  loading.value = true
  loadError.value = ''
  try {
    const result = await fetchPlans(filters, controller.signal)
    if (version !== loadVersion || controller.signal.aborted) return
    plans.value = result
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

watch([() => filters.name, () => filters.status], () => { void loadPlans() }, { immediate: true })
onBeforeUnmount(() => loadController?.abort())

function statusTagType(status: PlanStatus): 'default' | 'success' | 'warning' | 'info' {
  if (status === 'running') return 'warning'
  if (status === 'completed') return 'info'
  return 'default'
}

const columns: DataTableColumns<PlanRow> = [
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
  { title: '最近运行结果', key: 'lastRunResult', width: 180, ellipsis: { tooltip: true } },
  {
    title: '操作',
    key: 'actions',
    width: 260,
    fixed: 'right',
    render: (row) => {
      const action = getPlanAction(row.status)
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
        h(NButton, { size: 'small', secondary: true, type: 'primary', onClick: () => handlePlanAction(row) }, { default: () => action.label }),
        row.status === 'not_started' ? h('span', { class: 'plan-row-actions__delete' }, [
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
    </div>

    <div v-if="prototypeNotice" class="prototype-notice" role="status">
      {{ prototypeNotice }}
      <n-button text type="primary" @click="prototypeNotice = ''">关闭</n-button>
    </div>

    <div v-if="loadError" class="plan-load-error" role="alert">
      <span>{{ loadError }}</span>
      <n-button text type="primary" @click="loadPlans">重试</n-button>
    </div>

    <div v-else class="plan-table-region">
      <n-data-table
        :columns="columns"
        :data="plans"
        :loading="loading"
        :row-key="(row: PlanRow) => row.id"
        :scroll-x="1458"
        :single-line="false"
        striped
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
  color: var(--n-text-color-2);
  line-height: 1.7;
}

.plan-filters {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
}

.plan-name-filter {
  width: 280px;
}

.plan-status-filter {
  width: 180px;
}

.prototype-notice {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 16px;
  color: var(--n-text-color-2);
  line-height: 1.6;
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
  color: var(--n-text-color-2);
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
