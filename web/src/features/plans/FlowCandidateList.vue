<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NButton, NEl, NEmpty, NInput, NSpin, NTag, NText, NVirtualList } from 'naive-ui'

import { flowSelectionLabels } from './selection'
import {
  CANDIDATE_ITEM_SIZE,
  candidateDetail,
  candidateDetailTitle,
  candidateMeta,
  candidateStatus,
  candidateTitle,
  templateGroupName,
} from './presentation'
import type { FlowCandidate, FlowSource } from './types'

const props = defineProps<{
  source: FlowSource
  items: FlowCandidate[]
  selectedKey: string | null
  requestKey: string
	accountName: string
	loading: boolean
	loadingMore: boolean
	error: string
	hasMore: boolean
	total: number
}>()

const emit = defineEmits<{
  select: [key: string]
	queryChange: [query: string]
	loadMore: []
	retry: []
}>()

const query = ref('')
const searchFieldRef = ref<HTMLElement | null>(null)

const title = computed(() => flowSelectionLabels[props.source])

watch(() => props.requestKey, () => {
  query.value = ''
})

function updateQuery(value: string) {
	query.value = value
	emit('queryChange', value)
}

function handleScroll(event: Event) {
  const target = event.target as HTMLElement | null
  if (!target) return
  if (target.scrollTop + target.clientHeight >= target.scrollHeight - 24 && props.hasMore && !props.loading && !props.loadingMore) {
	emit('loadMore')
  }
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
		  :value="query"
          clearable
		  :placeholder="`搜索${title}名称`"
          :aria-label="`搜索${title}`"
		  @update:value="updateQuery"
        />
      </div>
		<n-text depth="3">{{ accountName }} · 已加载 {{ items.length }} / {{ total }}</n-text>
    </div>

	<div v-if="loading && items.length === 0" class="candidate-state" aria-live="polite">
	  <n-spin size="small" />
	  <n-text depth="3">正在读取{{ title }}…</n-text>
	</div>

	<div v-else-if="error && items.length === 0" class="candidate-state" aria-live="polite">
	  <n-empty :description="error" />
	  <n-button secondary size="small" @click="emit('retry')">重新加载</n-button>
	</div>

	<n-empty v-else-if="items.length === 0" class="candidate-empty" :description="`没有匹配的${title}`" />

    <n-virtual-list
      v-else
      class="candidate-virtual-list"
	  :items="items"
		  :item-size="CANDIDATE_ITEM_SIZE"
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
			<n-tag
			  v-if="templateGroupName(item as FlowCandidate)"
			  size="small"
			  type="info"
			  :bordered="false"
			>
			  {{ templateGroupName(item as FlowCandidate) }}
			</n-tag>
			<n-tag
			  v-if="selectedKey === item.key || candidateStatus(item as FlowCandidate)"
			  size="small"
			  :type="selectedKey === item.key ? 'success' : 'default'"
			  :bordered="false"
			>
			  {{ selectedKey === item.key ? '已选择' : candidateStatus(item as FlowCandidate) }}
			</n-tag>
		  </span>
		  <span class="candidate-row__meta">{{ candidateMeta(item as FlowCandidate) }}</span>
		  <span
			class="candidate-row__detail"
			:class="{ 'candidate-row__detail--remark': item.kind === 'template' }"
			:title="candidateDetailTitle(item as FlowCandidate)"
		  >{{ candidateDetail(item as FlowCandidate) }}</span>
        </button>
      </template>
    </n-virtual-list>

	<div v-if="items.length > 0" class="candidate-footer" aria-live="polite">
	  <template v-if="loadingMore">
        <n-spin size="small" />
        <n-text depth="3">正在追加下一批…</n-text>
      </template>
	  <template v-else-if="error">
		<n-text type="error">{{ error }}</n-text>
		<n-button text type="primary" size="small" @click="emit('retry')">重试本页</n-button>
	  </template>
	  <n-text v-else-if="hasMore" depth="3">滚动到底加载更多</n-text>
	  <n-text v-else depth="3">已显示全部 {{ items.length }} 项</n-text>
    </div>
  </n-el>
</template>

<style scoped>
.candidate-picker {
  width: 100%;
  min-width: 0;
	min-height: 574px;
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
	height: 480px;
}

.candidate-row {
  display: flex;
  flex-direction: column;
  justify-content: center;
  width: 100%;
	height: 96px;
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
	transition: background-color 120ms ease, border-color 120ms ease;
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
	flex: 1 1 auto;
	min-width: 0;
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

.candidate-row__detail--remark {
	display: -webkit-box;
	line-height: 1.35;
	white-space: normal;
	-webkit-box-orient: vertical;
	-webkit-line-clamp: 2;
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
	min-height: 480px;
	padding-top: 160px;
}

.candidate-state {
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	gap: 12px;
	min-height: 480px;
}

@media (prefers-reduced-motion: reduce) {
	.candidate-row {
		transition: none;
	}
}
</style>
