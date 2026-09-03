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
  useThemeVars,
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
  show: boolean
  disabled?: boolean
  confirmText?: string
}>()
const emit = defineEmits<{ 'update:show': [value: boolean], saved: [source: HistoryDataSource] }>()

const themeVars = useThemeVars()
const pageState = ref<HistoryCandidatePage | null>(null)
const loading = ref(false)
const saving = ref(false)
const page = ref(1)
const selectedKey = ref('')
const pathMode = ref<'default' | 'override'>('override')
const error = ref('')
let controller: AbortController | null = null
let saveKey = crypto.randomUUID()

const currentSource = computed(() => props.scope === 'path'
  ? pageState.value?.pathSource
  : pageState.value?.defaultSource)
const candidates = computed(() => pageState.value?.items ?? [])
const pageCount = computed(() => Math.max(1, Math.ceil((pageState.value?.total ?? 0) / (pageState.value?.pageSize ?? 20))))
const pickerStyle = computed(() => ({
  '--picker-border-color': themeVars.value.borderColor,
  '--picker-active-color': themeVars.value.primaryColor,
  '--picker-secondary-text-color': themeVars.value.textColor3,
}))

// loadCandidates 读取可选业务数据；请求切换时中止旧调用，避免跨路径回写。
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
    if (props.scope === 'path') pathMode.value = source?.mode === 'default' && Boolean(result.defaultSource?.summary) ? 'default' : 'override'
    selectedKey.value = source?.summary?.candidateKey ?? ''
  }
  catch (caught) {
    if (active.signal.aborted) return
    error.value = caught instanceof HistoryDataApiError ? caught.message : '暂时无法读取可选业务数据，请重试'
  }
  finally {
    if (controller === active) {
      controller = null
      loading.value = false
    }
  }
}

// selectCandidate 只保留后端不透明候选键，不把目标实例字段作为保存参数。
function selectCandidate(candidate: HistoryCandidate) {
  if (!candidate.snapshotAvailable) return
  selectedKey.value = candidate.candidateKey
  if (props.scope === 'path') pathMode.value = 'override'
}

// confirmSelection 按作用域保存计划统一数据或路径单独指定的基础表单数据。
async function confirmSelection() {
  if (saving.value || props.disabled) return
  if ((props.scope === 'default' || pathMode.value === 'override') && !selectedKey.value) {
    error.value = '请选择一条可读取的业务数据'
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
    emit('update:show', false)
    emit('saved', source)
  }
  catch (caught) {
    error.value = caught instanceof HistoryDataApiError ? caught.message : '基础表单数据保存失败，请重试'
    if (caught instanceof HistoryDataApiError && caught.code === 'HISTORY_REVISION_CONFLICT') {
      await loadCandidates(page.value)
    }
  }
  finally {
    saving.value = false
  }
}

// changePage 翻页时保留当前选择模式和已选业务数据。
function changePage(value: number) {
  void loadCandidates(value)
}

watch(() => [props.planId, props.pathId, props.scope], () => {
  pageState.value = null
  selectedKey.value = ''
  saveKey = crypto.randomUUID()
})
watch(() => props.show, (open) => {
  if (open) void loadCandidates(1)
  else controller?.abort()
}, { immediate: true })

onBeforeUnmount(() => controller?.abort())
</script>

<template>
  <n-modal :show="show" @update:show="value => emit('update:show', value)">
    <n-card class="base-form-data-picker" data-testid="base-form-data-picker" title="选择基础表单数据" :style="pickerStyle" :aria-busy="loading">
      <div class="base-form-data-picker__body">
        <n-radio-group v-if="scope === 'path'" v-model:value="pathMode" size="small">
          <n-radio-button value="default">沿用计划统一数据</n-radio-button>
          <n-radio-button value="override">本路径单独指定</n-radio-button>
        </n-radio-group>
        <n-alert v-if="error" type="error" :show-icon="false">{{ error }}</n-alert>
        <div v-if="loading" class="base-form-data-picker__loading"><n-spin size="small" />正在读取可选业务数据</div>
        <template v-else-if="scope === 'default' || pathMode === 'override'">
          <div v-if="candidates.length" class="base-form-data-picker__candidates">
            <button
              v-for="candidate in candidates"
              :key="candidate.candidateKey"
              type="button"
              class="base-form-data-picker__candidate"
              :data-completeness="candidate.completeness"
              :class="{ 'base-form-data-picker__candidate--selected': selectedKey === candidate.candidateKey }"
              :disabled="!candidate.snapshotAvailable"
              @click="selectCandidate(candidate)"
            >
              <div class="base-form-data-picker__candidate-head">
                <strong>{{ candidate.instanceTitle || candidate.businessSummary || candidate.formName || candidate.flowName }}</strong>
                <n-tag size="small" :bordered="false" :type="candidate.completeness === 'complete' ? 'success' : 'warning'">{{ candidate.statusName || candidate.status }}</n-tag>
              </div>
              <p>{{ candidate.formName || candidate.flowName }}</p>
              <small>{{ [candidate.initiator, candidate.companyName, candidate.createdAt].filter(Boolean).join(' · ') || '目标摘要字段不完整' }}</small>
              <n-alert v-if="candidate.integrityNotice" type="warning" :show-icon="false">{{ candidate.integrityNotice }}</n-alert>
            </button>
          </div>
          <n-empty v-else description="请先在目标平台发起一次该流程并填写业务数据，再回来刷新" />
          <n-pagination v-if="pageCount > 1" :page="page" :page-count="pageCount" @update:page="changePage" />
        </template>
        <n-alert v-else type="info" :show-icon="false">当前路径将沿用计划统一的基础表单数据。</n-alert>
      </div>
      <template #footer>
        <n-space justify="end">
          <n-button :disabled="saving" @click="emit('update:show', false)">取消</n-button>
          <n-button type="primary" :loading="saving" :disabled="disabled || loading || ((scope === 'default' || pathMode === 'override') && !selectedKey)" @click="confirmSelection">
            {{ confirmText || '确认' }}
          </n-button>
        </n-space>
      </template>
    </n-card>
  </n-modal>
</template>

<style scoped>
.base-form-data-picker {
  width: min(760px, 94vw);
}
.base-form-data-picker__body {
  display: grid;
  gap: 14px;
  max-height: min(68vh, 720px);
  overflow: auto;
}
.base-form-data-picker__loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 160px;
}
.base-form-data-picker__candidates {
  display: grid;
  gap: 10px;
}
.base-form-data-picker__candidate {
  width: 100%;
  padding: 12px 14px;
  color: inherit;
  font: inherit;
  text-align: left;
  border: 1px solid var(--picker-border-color);
  border-radius: 4px;
  background: transparent;
  cursor: pointer;
}
.base-form-data-picker__candidate-head {
  display: flex;
  align-items: center;
  gap: 8px;
}
.base-form-data-picker__candidate p {
  margin: 5px 0 2px;
}
.base-form-data-picker__candidate small {
  color: var(--picker-secondary-text-color);
}
.base-form-data-picker__candidate:hover,
.base-form-data-picker__candidate--selected {
  border-color: var(--picker-active-color);
}
.base-form-data-picker__candidate:disabled {
  cursor: not-allowed;
  opacity: .65;
}
.base-form-data-picker__candidate :deep(.n-alert) {
  margin-top: 8px;
}
</style>
