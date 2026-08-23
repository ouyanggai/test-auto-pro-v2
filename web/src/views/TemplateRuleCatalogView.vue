<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref } from 'vue'
import { NAlert, NButton, NCard, NDataTable, NDescriptions, NDescriptionsItem, NInput, NPagination, NProgress, NSpin, NTag, useMessage, type DataTableColumns } from 'naive-ui'

import {
  createTemplateRuleAnalysis,
  fetchLatestTemplateRuleAnalysis,
  fetchTemplateRuleAnalysis,
  fetchTemplateRuleCatalog,
  fetchTemplateRuleSummary,
  type TemplateRuleAnalysisJob,
  type TemplateRuleCatalogItem,
  type TemplateRuleCatalogSummary,
} from '../features/template-catalog/api'

const message = useMessage()
const account = ref('欧阳改')
const summary = ref<TemplateRuleCatalogSummary | null>(null)
const items = ref<TemplateRuleCatalogItem[]>([])
const total = ref(0)
const pageNumber = ref(1)
const pageSize = 50
const query = ref('')
const loading = ref(true)
const creating = ref(false)
const errorMessage = ref('')
const job = ref<TemplateRuleAnalysisJob | null>(null)
let pollTimer: number | undefined
let requestVersion = 0

const active = computed(() => job.value?.state === 'queued' || job.value?.state === 'running')
const progress = computed(() => {
  if (!job.value?.total) return job.value?.state === 'finished' ? 100 : 0
  return job.value.state === 'finished' ? 100 : Math.min(99, Math.floor(job.value.accounted * 100 / job.value.total))
})

// loadCatalog 并行读取本地摘要、分页规则和上次任务；目录读取本身不访问目标平台。
async function loadCatalog() {
  const version = ++requestVersion
  loading.value = true
  errorMessage.value = ''
  try {
    const [nextSummary, page, latest] = await Promise.all([
      fetchTemplateRuleSummary(),
      fetchTemplateRuleCatalog(pageNumber.value, pageSize, query.value),
      fetchLatestTemplateRuleAnalysis(account.value).catch(() => null),
    ])
    if (version !== requestVersion) return
    summary.value = nextSummary
    items.value = page.items
    total.value = page.total
    job.value = latest
    if (latest && (latest.state === 'queued' || latest.state === 'running')) await refreshJob(latest.id, version)
  } catch (caught) {
    if (version === requestVersion) errorMessage.value = caught instanceof Error ? caught.message : '模板规则目录读取失败'
  } finally {
    if (version === requestVersion) loading.value = false
  }
}

// searchCatalog 从第一页执行目录搜索，避免沿用旧页码得到假空状态。
function searchCatalog() {
  pageNumber.value = 1
  void loadCatalog()
}

// changeCatalogPage 按服务端真实总数切换分页，浏览器只保留当前页摘要。
function changeCatalogPage(page: number) {
  pageNumber.value = page
  void loadCatalog()
}

// refreshJob 只以服务端真实计数推进任务，完成后刷新目录数据。
async function refreshJob(id: string, version = requestVersion) {
  const next = await fetchTemplateRuleAnalysis(id)
  if (version !== requestVersion || next.id !== id) return
  job.value = next
  if (next.state === 'queued' || next.state === 'running') {
    if (pollTimer) window.clearTimeout(pollTimer)
    pollTimer = window.setTimeout(() => { void refreshJob(id, version).catch(reportError) }, 1000)
    return
  }
  await loadCatalog()
}

// startAnalysis 创建增量、全量或失败重试任务，账号仅用于目标平台已有会话权限范围。
async function startAnalysis(mode: TemplateRuleAnalysisJob['mode']) {
  creating.value = true
  errorMessage.value = ''
  const version = ++requestVersion
  try {
    const next = await createTemplateRuleAnalysis(account.value.trim(), mode)
    job.value = next
    message.success('模板规则分析任务已创建')
    await refreshJob(next.id, version)
  } catch (caught) {
    errorMessage.value = caught instanceof Error ? caught.message : '模板规则分析任务创建失败'
  } finally {
    creating.value = false
  }
}

// reportError 将轮询异常收敛为页面可重试提示，旧轮询不会覆盖新页面数据。
function reportError(caught: unknown) {
  errorMessage.value = caught instanceof Error ? caught.message : '模板规则分析状态读取失败'
}

const columns: DataTableColumns<TemplateRuleCatalogItem> = [
  { title: '流程', key: 'flowName', minWidth: 180, ellipsis: { tooltip: true }, render: row => row.flowName || row.flowCode },
  { title: '编码', key: 'flowCode', width: 180, ellipsis: { tooltip: true } },
  { title: '页面类型', key: 'renderType', width: 110, render: row => h(NTag, { size: 'small', bordered: false, type: row.renderType === 'unknown' ? 'warning' : 'default' }, { default: () => ({ formmaking: 'FormMaking', vue_custom: 'Vue 页面', unknown: '待识别' }[row.renderType]) }) },
  { title: '分析结果', key: 'status', width: 110, render: row => h(NTag, { size: 'small', bordered: false, type: row.status === 'complete' ? 'success' : row.status === 'failed' || row.status === 'blocked' ? 'error' : 'warning' }, { default: () => ({ complete: '已覆盖', needs_attention: '需处理', blocked: '已阻断', failed: '失败' }[row.status]) }) },
  { title: '问题', key: 'issues', minWidth: 260, ellipsis: { tooltip: true }, render: row => row.issues.length ? row.issues.join('；') : '无' },
]

onMounted(() => { void loadCatalog() })
onBeforeUnmount(() => {
  requestVersion++
  if (pollTimer) window.clearTimeout(pollTimer)
})
</script>

<template>
  <section class="template-catalog">
    <header class="template-catalog__header">
      <div>
        <router-link to="/settings">返回系统设置</router-link>
        <h1>模板规则目录</h1>
        <p>本地保存流程页面、组件和条件规则；计划配置只读取已分析目录。</p>
      </div>
      <div class="template-catalog__actions">
        <n-input v-model:value="account" aria-label="分析账号" :disabled="active || creating" />
        <n-button :loading="creating" :disabled="loading || active" @click="startAnalysis('incremental')">增量分析</n-button>
        <n-button :loading="creating" :disabled="loading || active" @click="startAnalysis('full')">全量重分析</n-button>
        <n-button type="warning" :loading="creating" :disabled="loading || active" @click="startAnalysis('retry')">重试失败项</n-button>
      </div>
    </header>

    <n-alert v-if="errorMessage" type="error" :show-icon="false">{{ errorMessage }}</n-alert>
    <n-spin :show="loading">
      <n-card v-if="job" title="分析进度" size="small">
        <div class="template-catalog__job">
          <span>{{ job.message || '等待分析' }}</span>
          <strong>已对账 {{ job.accounted }} / {{ job.total }}</strong>
        </div>
        <n-progress type="line" :percentage="progress" :show-indicator="true" />
        <div class="template-catalog__counts">已列出 {{ job.listed }}；已覆盖 {{ job.complete }}；需处理 {{ job.needsAttention }}；已阻断 {{ job.blocked }}；失败 {{ job.failed }}；未列出 {{ job.unlisted }}</div>
        <n-alert v-if="job.state === 'finished' && job.outcome !== 'success'" :type="job.outcome === 'failed' ? 'error' : 'warning'" :show-icon="false">
          {{ job.outcome === 'failed' ? '分析失败' : '分析已结束，但存在需处理项' }}
          <span v-if="job.failures.length">：{{ job.failures.map(item => `${item.stage}${item.page ? ` 第${item.page}页` : ''}：${item.reason}`).join('；') }}</span>
        </n-alert>
      </n-card>

      <n-descriptions v-if="summary" class="template-catalog__summary" :column="4" bordered size="small" label-placement="left">
        <n-descriptions-item label="目录模板">{{ summary.catalogTotal }}</n-descriptions-item>
        <n-descriptions-item label="FormMaking">{{ summary.formmaking }}</n-descriptions-item>
        <n-descriptions-item label="Vue 页面">{{ summary.vueCustom }}</n-descriptions-item>
        <n-descriptions-item label="待识别">{{ summary.unknown }}</n-descriptions-item>
        <n-descriptions-item label="已覆盖">{{ summary.complete }}</n-descriptions-item>
        <n-descriptions-item label="需处理">{{ summary.needsAttention }}</n-descriptions-item>
        <n-descriptions-item label="已阻断">{{ summary.blocked }}</n-descriptions-item>
        <n-descriptions-item label="失败">{{ summary.failed }}</n-descriptions-item>
        <n-descriptions-item label="模板实际组件">{{ Object.keys(summary.components).length }}</n-descriptions-item>
        <n-descriptions-item label="全局注册组件">{{ summary.registeredComponents }}</n-descriptions-item>
      </n-descriptions>

      <n-card title="已保存规则" size="small">
        <n-input v-model:value="query" clearable placeholder="搜索流程名称或编码" @keyup.enter="searchCatalog" />
        <n-data-table :columns="columns" :data="items" :bordered="false" :single-line="false" :max-height="560" />
        <div class="template-catalog__footer">
          <p class="template-catalog__total">共 {{ total }} 条已保存规则</p>
          <n-pagination :page="pageNumber" :page-size="pageSize" :item-count="total" @update:page="changeCatalogPage" />
        </div>
      </n-card>
    </n-spin>
  </section>
</template>

<style scoped>
.template-catalog { display: grid; gap: 16px; padding: 24px; }
.template-catalog__header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.template-catalog__header h1 { margin: 6px 0 4px; }
.template-catalog__header p { margin: 0; color: var(--n-text-color-3); }
.template-catalog__actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.template-catalog__actions :deep(.n-input) { width: 116px; }
.template-catalog__job { display: flex; justify-content: space-between; gap: 12px; margin-bottom: 8px; }
.template-catalog__counts, .template-catalog__total { margin: 8px 0 0; color: var(--n-text-color-3); }
.template-catalog__footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.template-catalog__summary { margin: 0; }
@media (max-width: 860px) { .template-catalog__header { flex-direction: column; } .template-catalog__actions { width: 100%; } }
</style>
