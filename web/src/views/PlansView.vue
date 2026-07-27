<script setup lang="ts">
import { computed, h, reactive, ref } from 'vue'
import {
  NButton,
  NDataTable,
  NInput,
  NSelect,
  NTag,
  type DataTableColumns,
  type SelectOption,
} from 'naive-ui'
import { useRouter } from 'vue-router'

import { filterPlans, getPlanAction } from '../features/plans/logic'
import { mockPlans, planStatusLabels, planStatusOptions } from '../features/plans/mock'
import type { PlanFilters, PlanRow, PlanStatus } from '../features/plans/types'

const router = useRouter()
const filters = reactive<PlanFilters>({ name: '', status: null })
const prototypeNotice = ref('')

const filteredPlans = computed(() => filterPlans(mockPlans, filters))
const statusOptions: SelectOption[] = planStatusOptions
const hasFilters = computed(() => Boolean(filters.name || filters.status))

function clearFilters() {
  filters.name = ''
  filters.status = null
}

function showPrototypeNotice(plan: PlanRow) {
  const action = getPlanAction(plan.status)
  prototypeNotice.value = `“${plan.name}”的“${action.label}”当前仅用于静态原型展示，真实业务将在后续功能接入。`
}

function statusTagType(status: PlanStatus): 'default' | 'success' | 'warning' | 'info' {
  if (status === 'ready') return 'success'
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
    render: (row) => row.scheduledAt ?? '未设置',
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
    width: 120,
    fixed: 'right',
    render: (row) => {
      const action = getPlanAction(row.status)
      return h(NButton, { text: true, type: 'primary', onClick: () => showPrototypeNotice(row) }, { default: () => action.label })
    },
  },
]
</script>

<template>
  <section class="plan-page">
    <header class="page-heading">
      <div>
        <h1>测试计划</h1>
        <p>查看并筛选测试计划；当前数据仅用于静态交互确认。</p>
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

    <div class="plan-table-region">
      <n-data-table
        :columns="columns"
        :data="filteredPlans"
        :row-key="(row: PlanRow) => row.id"
        :scroll-x="1370"
        :single-line="false"
        striped
      />
    </div>
  </section>
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
</style>
