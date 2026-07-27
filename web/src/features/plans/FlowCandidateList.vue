<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { NEl, NEmpty, NInput, NSpin, NTag, NText, NVirtualList } from 'naive-ui'

import { filterFlowCandidates, flowSelectionLabels, takeCandidateBatches } from './selection'
import type { FlowCandidate, FlowSource } from './types'

const props = defineProps<{
  source: FlowSource
  items: FlowCandidate[]
  selectedKey: string | null
  requestKey: string
}>()

const emit = defineEmits<{
  select: [key: string]
}>()

const query = ref('')
const batchCount = ref(1)
const loading = ref(false)
const searchFieldRef = ref<HTMLElement | null>(null)
let requestVersion = 0

const title = computed(() => flowSelectionLabels[props.source])
const filteredItems = computed(() => filterFlowCandidates(props.items, query.value))
const visibleItems = computed(() => takeCandidateBatches(filteredItems.value, batchCount.value))
const hasMore = computed(() => visibleItems.value.length < filteredItems.value.length)

function resetList() {
  requestVersion += 1
  batchCount.value = 1
  loading.value = false
}

watch(() => props.requestKey, () => {
  query.value = ''
  resetList()
})

watch(query, resetList)

async function loadMore() {
  if (loading.value || !hasMore.value) return
  const version = ++requestVersion
  loading.value = true
  await nextTick()
  await new Promise<void>((resolve) => window.requestAnimationFrame(() => resolve()))
  if (version !== requestVersion) return
  batchCount.value += 1
  loading.value = false
}

function handleScroll(event: Event) {
  const target = event.target as HTMLElement | null
  if (!target) return
  if (target.scrollTop + target.clientHeight >= target.scrollHeight - 24) {
    void loadMore()
  }
}

function candidateTitle(candidate: FlowCandidate): string {
  if (candidate.kind === 'template') return candidate.flowName
  if (candidate.kind === 'submitted') return candidate.name
  return candidate.flowInstanceName
}

function candidateStatus(candidate: FlowCandidate): string {
  if (candidate.kind === 'template') return candidate.statusText
  if (candidate.kind === 'submitted') return candidate.status
  return candidate.statusName
}

function candidateMeta(candidate: FlowCandidate): string {
  if (candidate.kind === 'template') return `${candidate.typeName} · ${candidate.groupName}`
  if (candidate.kind === 'submitted') return `提交时间 ${candidate.createDate} · 当前节点 ${candidate.currentNodeName}`
  return `发起人 ${candidate.initiator} · 提交时间 ${candidate.initiatorDate}`
}

function candidateDetail(candidate: FlowCandidate): string {
  if (candidate.kind === 'template') return `更新于 ${candidate.updateTime}`
  if (candidate.kind === 'submitted') return `当前处理人 ${candidate.currentAuditUserNames}`
  return `实例编号 ${candidate.flowInstanceId}`
}

function getSearchElement(): HTMLInputElement | null {
  return searchFieldRef.value?.querySelector('input') ?? null
}

function focusSearch() {
  const input = getSearchElement()
  if (!input) return
  input.focus({ preventScroll: true })
}

defineExpose({ getSearchElement, focusSearch })
</script>

<template>
  <n-el class="candidate-picker">
    <div class="candidate-toolbar">
      <div ref="searchFieldRef" class="candidate-search-field">
        <n-input
          v-model:value="query"
          clearable
          :placeholder="`搜索${title}名称或状态`"
          :aria-label="`搜索${title}`"
        />
      </div>
      <n-text depth="3">{{ items[0]?.accountName }} · 本地静态数据 · {{ filteredItems.length }} 项</n-text>
    </div>

    <n-empty v-if="filteredItems.length === 0" class="candidate-empty" :description="`没有匹配的${title}`" />

    <n-virtual-list
      v-else
      class="candidate-virtual-list"
      :items="visibleItems"
      :item-size="84"
      key-field="key"
      @scroll="handleScroll"
    >
      <template #default="{ item }">
        <button
          type="button"
          class="candidate-row"
          :class="{ 'candidate-row--selected': selectedKey === item.key }"
          :aria-pressed="selectedKey === item.key"
          @click="emit('select', item.key)"
        >
          <span class="candidate-row__heading">
            <strong>{{ candidateTitle(item as FlowCandidate) }}</strong>
            <n-tag size="small" :type="selectedKey === item.key ? 'success' : 'default'" :bordered="false">
              {{ selectedKey === item.key ? '已选择' : candidateStatus(item as FlowCandidate) }}
            </n-tag>
          </span>
          <span class="candidate-row__meta">{{ candidateMeta(item as FlowCandidate) }}</span>
          <span class="candidate-row__detail">{{ candidateDetail(item as FlowCandidate) }}</span>
        </button>
      </template>
    </n-virtual-list>

    <div v-if="filteredItems.length > 0" class="candidate-footer" aria-live="polite">
      <template v-if="loading">
        <n-spin size="small" />
        <n-text depth="3">正在追加下一批…</n-text>
      </template>
      <n-text v-else-if="hasMore" depth="3">滚动到底加载更多</n-text>
      <n-text v-else depth="3">已显示全部 {{ filteredItems.length }} 项</n-text>
    </div>
  </n-el>
</template>

<style scoped>
.candidate-picker {
  width: 100%;
  min-width: 0;
  min-height: 348px;
}

.candidate-toolbar {
  display: flex;
  align-items: center;
  gap: 16px;
  min-height: 56px;
  padding: 10px 12px;
  border-bottom: 1px solid var(--divider-color);
}

.candidate-search-field {
  width: min(100%, 360px);
}

.candidate-search-field .n-input {
  width: 100%;
}

.candidate-toolbar .n-text {
  margin-left: auto;
  white-space: nowrap;
}

.candidate-virtual-list {
  height: 252px;
}

.candidate-row {
  display: flex;
  flex-direction: column;
  justify-content: center;
  width: 100%;
  height: 84px;
  padding: 10px 16px 10px 13px;
  overflow: hidden;
  color: var(--text-color-1);
  font: inherit;
  text-align: left;
  background: transparent;
  border: 0;
  border-bottom: 1px solid var(--divider-color);
  border-left: 3px solid transparent;
  cursor: pointer;
}

.candidate-row:hover {
  background: var(--hover-color);
}

.candidate-row:focus-visible {
  outline: 2px solid var(--primary-color);
  outline-offset: -2px;
}

.candidate-row--selected {
  background: var(--table-color-hover);
  border-left-color: var(--primary-color);
}

.candidate-row__heading {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.candidate-row__heading strong {
  overflow: hidden;
  font-size: 14px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.candidate-row__heading .n-tag {
  flex: 0 0 auto;
}

.candidate-row__meta,
.candidate-row__detail {
  margin-top: 3px;
  overflow: hidden;
  color: var(--text-color-3);
  font-size: 12px;
  line-height: 1.25;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.candidate-footer {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 38px;
  border-top: 1px solid var(--divider-color);
}

.candidate-empty {
  min-height: 252px;
  padding-top: 72px;
}
</style>
