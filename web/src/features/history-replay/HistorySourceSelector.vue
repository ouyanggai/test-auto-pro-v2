<script setup lang="ts">
import {
  NAlert,
  NButton,
  NCard,
  NEmpty,
  NModal,
  NPagination,
  NRadioButton,
  NRadioGroup,
  NSpace,
  NSpin,
  NTag,
} from 'naive-ui'
import { computed, onBeforeUnmount, ref, watch } from 'vue'

import {
  fetchHistoryCandidates,
  HistoryDataApiError,
  saveDefaultHistorySource,
  savePathHistorySource,
} from './api'
import type { HistoryCandidate, HistoryCandidatePage, HistoryDataSource } from './types'

const props = defineProps<{
  planId: string
  pathId?: string
  scope: 'default' | 'path'
  disabled?: boolean
}>()
const emit = defineEmits<{ saved: [source: HistoryDataSource] }>()

const pageState = ref<HistoryCandidatePage | null>(null)
const loading = ref(false)
const saving = ref(false)
const modalOpen = ref(false)
const page = ref(1)
const selectedKey = ref('')
const pathMode = ref<'default' | 'override'>('default')
const error = ref('')
let controller: AbortController | null = null
let saveKey = crypto.randomUUID()

const currentSource = computed(() => props.scope === 'path'
  ? pageState.value?.pathSource
  : pageState.value?.defaultSource)
const candidates = computed(() => pageState.value?.items ?? [])
const pageCount = computed(() => Math.max(1, Math.ceil((pageState.value?.total ?? 0) / (pageState.value?.pageSize ?? 20))))
const sourceTitle = computed(() => {
  const source = currentSource.value
  if (!source?.summary) return props.scope === 'path' && source?.mode === 'default' ? '继承计划默认来源' : '尚未选择历史来源'
  return source.summary.instanceTitle || source.summary.businessSummary || source.summary.formName || source.summary.flowName || '历史来源'
})
const sourceDetail = computed(() => {
  const source = currentSource.value
  if (!source?.summary) return props.scope === 'path' && source?.mode === 'default'
    ? '计划默认来源尚未设置'
    : '请选择当前账号可见的同流程历史实例'
  const summary = source.summary
  return [summary.formName || summary.flowName, summary.statusName, summary.createdAt].filter(Boolean).join(' · ')
})

// loadCandidates 读取候选和来源摘要；请求切换时中止旧调用，避免跨路径回写。
async function loadCandidates(nextPage = 1) {
  controller?.abort()
  const active = new AbortController()
  controller = active
  loading.value = true
  error.value = ''
  try {
    const result = await fetchHistoryCandidates(props.planId, {
      pathId: props.scope === 'path' ? props.pathId : undefined,
      page: nextPage,
      pageSize: 20,
      signal: active.signal,
    })
    if (active.signal.aborted) return
    pageState.value = result
    page.value = result.page
    const source = props.scope === 'path' ? result.pathSource : result.defaultSource
    pathMode.value = source?.mode === 'override' ? 'override' : 'default'
    selectedKey.value = source?.summary?.candidateKey ?? ''
  }
  catch (caught) {
    if (active.signal.aborted) return
    error.value = caught instanceof HistoryDataApiError ? caught.message : '暂时无法读取历史来源，请重试'
  }
  finally {
    if (controller === active) {
      controller = null
      loading.value = false
    }
  }
}

// openSelector 打开弹窗并刷新目标候选，避免使用过期可见性。
async function openSelector() {
  modalOpen.value = true
  await loadCandidates(1)
}

// selectCandidate 只保留后端不透明候选键，不把目标实例字段作为保存参数。
function selectCandidate(candidate: HistoryCandidate) {
  if (!candidate.snapshotAvailable) return
  selectedKey.value = candidate.candidateKey
  if (props.scope === 'path') pathMode.value = 'override'
}

// saveSelection 按作用域保存计划默认或路径继承/覆盖来源。
async function saveSelection() {
  if (saving.value || props.disabled) return
  if ((props.scope === 'default' || pathMode.value === 'override') && !selectedKey.value) {
    error.value = '请选择一条可读取的历史数据'
    return
  }
  saving.value = true
  error.value = ''
  try {
    const revision = currentSource.value?.revision ?? 0
    const source = props.scope === 'default'
      ? await saveDefaultHistorySource(props.planId, selectedKey.value, revision, saveKey)
      : await savePathHistorySource(props.planId, props.pathId || '', pathMode.value, selectedKey.value, revision, saveKey)
    if (!pageState.value) pageState.value = { items: [], page: 1, pageSize: 20, total: 0, hasMore: false }
    if (props.scope === 'default') pageState.value.defaultSource = source
    else pageState.value.pathSource = source
    saveKey = crypto.randomUUID()
    modalOpen.value = false
    emit('saved', source)
  }
  catch (caught) {
    error.value = caught instanceof HistoryDataApiError ? caught.message : '历史来源保存失败，请重试'
    if (caught instanceof HistoryDataApiError && caught.code === 'HISTORY_REVISION_CONFLICT') {
      await loadCandidates(page.value)
    }
  }
  finally {
    saving.value = false
  }
}

// changePage 翻页时保留当前来源模式和已选候选键。
function changePage(value: number) {
  void loadCandidates(value)
}

watch(() => [props.planId, props.pathId, props.scope], () => {
  pageState.value = null
  selectedKey.value = ''
  saveKey = crypto.randomUUID()
  void loadCandidates(1)
}, { immediate: true })

onBeforeUnmount(() => controller?.abort())
</script>

<template>
  <section class="history-source" data-testid="history-source-selector" :aria-busy="loading">
    <div class="history-source__summary">
      <div>
        <div class="history-source__title">
          <strong>{{ scope === 'default' ? '计划默认历史来源' : '路径历史来源' }}</strong>
          <n-tag v-if="currentSource" size="small" :bordered="false" :type="currentSource.dataStatus === 'ready' ? 'success' : currentSource.dataStatus === 'needs_input' ? 'warning' : 'default'">
            {{ currentSource.mode === 'override' ? '独立来源' : currentSource.mode === 'default' ? '继承默认' : '未选择' }}
          </n-tag>
        </div>
        <p>{{ sourceTitle }}</p>
        <small>{{ sourceDetail }}</small>
      </div>
      <n-button size="small" secondary :loading="loading && !modalOpen" :disabled="disabled" @click="openSelector">
        {{ currentSource?.summary ? '更换历史来源' : '选择历史来源' }}
      </n-button>
    </div>
    <n-alert
      v-for="issue in currentSource?.issues ?? []"
      :key="`${issue.code}:${issue.path ?? ''}:${issue.message}`"
      class="history-source__issue"
      :type="issue.blocking ? 'warning' : 'info'"
      :show-icon="false"
    >
      {{ issue.message }}
    </n-alert>
    <n-alert v-if="error && !modalOpen" type="error" :show-icon="false">
      {{ error }}
      <n-button text type="primary" @click="loadCandidates(page)">重试</n-button>
    </n-alert>

    <n-modal v-model:show="modalOpen">
      <n-card class="history-source__modal" :title="scope === 'default' ? '选择计划默认历史来源' : '选择路径历史来源'">
        <div class="history-source__body">
          <n-alert type="info" :show-icon="false">
            只显示当前计划账号可见、流程编码和目标原始表单/页面身份匹配的历史实例；保存不会修改目标实例。
          </n-alert>
          <n-radio-group v-if="scope === 'path'" v-model:value="pathMode" size="small">
            <n-radio-button value="default">继承计划默认来源</n-radio-button>
            <n-radio-button value="override">使用独立历史来源</n-radio-button>
          </n-radio-group>
          <n-alert v-if="error" type="error" :show-icon="false">{{ error }}</n-alert>
          <div v-if="loading" class="history-source__loading"><n-spin size="small" />正在读取目标历史实例</div>
          <template v-else-if="scope === 'default' || pathMode === 'override'">
            <div v-if="candidates.length" class="history-source__candidates">
              <button
                v-for="candidate in candidates"
                :key="candidate.candidateKey"
                type="button"
                class="history-source__candidate"
                :data-completeness="candidate.completeness"
                :class="{ 'history-source__candidate--selected': selectedKey === candidate.candidateKey }"
                :disabled="!candidate.snapshotAvailable"
                @click="selectCandidate(candidate)"
              >
                <div class="history-source__candidate-head">
                  <strong>{{ candidate.instanceTitle || candidate.businessSummary || candidate.formName || candidate.flowName }}</strong>
                  <n-tag size="small" :bordered="false" :type="candidate.completeness === 'complete' ? 'success' : 'warning'">{{ candidate.statusName || candidate.status }}</n-tag>
                </div>
                <p>{{ candidate.formName || candidate.flowName }} · {{ candidate.runtimeType === 'vue_custom' ? '自定义表单' : candidate.runtimeType === 'formmaking' ? 'FormMaking' : '运行时待核对' }}</p>
                <small>{{ [candidate.initiator, candidate.companyName, candidate.createdAt].filter(Boolean).join(' · ') || '目标摘要字段不完整' }}</small>
                <n-alert v-if="candidate.integrityNotice" type="warning" :show-icon="false">{{ candidate.integrityNotice }}</n-alert>
              </button>
            </div>
            <n-empty v-else description="请先在目标平台发起一次该流程并填写业务数据，再回来刷新" />
            <n-pagination v-if="pageCount > 1" :page="page" :page-count="pageCount" @update:page="changePage" />
          </template>
          <n-alert v-else type="info" :show-icon="false">
            当前路径将实时继承计划默认历史来源；以后更换计划默认值时无需逐路径修改。
          </n-alert>
        </div>
        <template #footer>
          <n-space justify="end">
            <n-button :disabled="saving" @click="modalOpen = false">取消</n-button>
            <n-button type="primary" :loading="saving" :disabled="disabled || loading || ((scope === 'default' || pathMode === 'override') && !selectedKey)" @click="saveSelection">确认来源</n-button>
          </n-space>
        </template>
      </n-card>
    </n-modal>
  </section>
</template>

<style scoped>
.history-source {
  min-width: 0;
}
.history-source__summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 14px;
  border: 1px solid var(--n-border-color, #e5e7eb);
  border-radius: 8px;
  background: var(--n-color, #fff);
}
.history-source__title,
.history-source__candidate-head {
  display: flex;
  align-items: center;
  gap: 8px;
}
.history-source__summary p,
.history-source__candidate p {
  margin: 5px 0 2px;
}
.history-source__summary small,
.history-source__candidate small {
  color: var(--n-text-color-3, #6b7280);
}
.history-source__issue {
  margin-top: 8px;
}
.history-source__modal {
  width: min(760px, 94vw);
}
.history-source__body {
  display: grid;
  gap: 14px;
  max-height: min(68vh, 720px);
  overflow: auto;
}
.history-source__loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 160px;
}
.history-source__candidates {
  display: grid;
  gap: 10px;
}
.history-source__candidate {
  width: 100%;
  padding: 12px 14px;
  color: inherit;
  text-align: left;
  border: 1px solid var(--n-border-color, #e5e7eb);
  border-radius: 8px;
  background: transparent;
  cursor: pointer;
}
.history-source__candidate:hover,
.history-source__candidate--selected {
  border-color: var(--n-color-target, #18a058);
  background: color-mix(in srgb, var(--n-color-target, #18a058) 7%, transparent);
}
.history-source__candidate:disabled {
  cursor: not-allowed;
  opacity: .65;
}
.history-source__candidate :deep(.n-alert) {
  margin-top: 8px;
}
</style>
