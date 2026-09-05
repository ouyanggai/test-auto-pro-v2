<script setup lang="ts">
import { NBadge, NButton, NCard, NCollapse, NCollapseItem, NEmpty, NSelect, NTag } from 'naive-ui'
import { computed, ref } from 'vue'

import type { HistoryDataIssue } from '../history-replay/types'
import type { PathConfigNodeView, PathConfigurationBranchPatch, PathConfigKeyField } from './types'

const props = defineProps<{
  keyFields: PathConfigKeyField[]
  issues: HistoryDataIssue[]
  branchPatches: PathConfigurationBranchPatch[]
  // nodeViews 是按节点切换的填写视图；selectedView 是当前视图的节点名称。
  nodeViews?: PathConfigNodeView[]
  selectedView?: string
}>()
const emit = defineEmits<{ 'update:selectedView': [value: string] }>()

// viewOptions 按路线顺序给出可切换的填写视图；只有一个视图时不展示切换器。
const viewOptions = computed(() => (props.nodeViews ?? []).map(view => ({
  label: view.isInitiator ? `${view.nodeName}（发起）` : view.nodeName,
  value: view.nodeName,
})))

const open = ref(true)

const decisiveFields = computed(() => props.keyFields.filter(field => field.decisive))
const otherFields = computed(() => props.keyFields.filter(field => !field.decisive))
const blockingIssues = computed(() => props.issues.filter(issue => issue.blocking))
const attentionCount = computed(() => blockingIssues.value.length + decisiveFields.value.length)
const expandedNames = computed(() => {
  const names: string[] = []
  if (decisiveFields.value.length) names.push('key')
  if (blockingIssues.value.length) names.push('issues')
  return names.length ? names : ['key']
})
const empty = computed(() => !decisiveFields.value.length && !blockingIssues.value.length
  && !props.branchPatches.length && !otherFields.value.length)

// fieldValueText 只展示目标返回的原值，空值明确写成未填写，不猜测默认值。
function fieldValueText(value: unknown): string {
  if (value === null || value === undefined || value === '') return '未填写'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

// candidateText 展示目标条件声明的真实候选值，超过四个只展示前四个。
function candidateText(candidates: unknown[] | undefined): string {
  if (!candidates?.length) return ''
  return candidates.slice(0, 4).map(fieldValueText).join(' / ')
}

// fieldLabel 优先显示目标中文标签，没有标签时使用不泄露内部路径的通用名称。
function fieldLabel(field: PathConfigKeyField): string {
  return field.label?.trim() || '条件字段'
}

// issueFieldText 把问题涉及的内部字段路径换成页面已有的中文名称和当前值。
function issueFieldText(issue: HistoryDataIssue): string {
  if (!issue.fields?.length) return ''
  return issue.fields.map(path => {
    const field = props.keyFields.find(item => item.path === path)
    if (!field) return '条件字段'
    return `${fieldLabel(field)}=${fieldValueText(field.current)}`
  }).join('；')
}

// fillHintText 说明这个条件字段由谁填：发起人自己填，还是要等到某个节点执行时自动带上。
// 目标条件求值只认本次写请求带上来的表单数据，所以后续节点填进去同样能决定分支走向。
function fillHintText(field: PathConfigKeyField): string {
  if (field.fillableAtStart) return '发起时由本页数据提交'
  if (field.fillNodeName) return `发起人无编辑权限，将在「${field.fillNodeName}」节点执行时自动填写`
  return '这条路线上没有节点有编辑权限，工具填不出这个值，需要人工确认'
}

// patchText 展示系统自动调整的中文字段、调整前值和调整后值。
function patchText(patch: PathConfigurationBranchPatch): string {
  const field = props.keyFields.find(item => item.path === patch.path)
  const label = field ? fieldLabel(field) : '条件字段'
  return `${label}：${fieldValueText(patch.before)} → ${fieldValueText(patch.after)}`
}
</script>

<template>
  <div class="form-hints" data-testid="form-data-hints">
    <n-badge class="form-hints__toggle" :value="attentionCount" :max="99" :show="!open && attentionCount > 0">
      <n-button
        secondary
        circle
        size="small"
        :title="open ? '收起路径关键信息' : '展开路径关键信息'"
        :aria-label="open ? '收起路径关键信息' : '展开路径关键信息'"
        :aria-expanded="open"
        @click="open = !open"
      >
        <span aria-hidden="true">{{ open ? '›' : '‹' }}</span>
      </n-button>
    </n-badge>

    <n-card v-if="open" size="small" class="form-hints__card" title="路径关键信息" :bordered="true">
      <div v-if="viewOptions.length > 1" class="form-hints__view">
        <span class="form-hints__view-label">按节点填写</span>
        <n-select
          size="small"
          :value="props.selectedView ?? ''"
          :options="viewOptions"
          aria-label="选择按哪个节点的字段权限填写"
          @update:value="value => emit('update:selectedView', String(value ?? ''))"
        />
        <small>只放开该节点有编辑权限的字段；只有后续节点才能填的字段在这个视图里隐藏，取值不会丢。</small>
      </div>

      <n-empty v-if="empty" size="small" description="当前路径没有需要优先核对的字段" />
      <n-collapse v-else :default-expanded-names="expandedNames">
        <n-collapse-item v-if="decisiveFields.length" name="key" title="决定当前路径的字段">
          <ul class="form-hints__list">
            <li v-for="field in decisiveFields" :key="field.path">
              <div class="form-hints__field-head">
                <strong>{{ fieldLabel(field) }}</strong>
                <n-tag size="tiny" :bordered="false" type="info">{{ fieldValueText(field.current) }}</n-tag>
              </div>
              <small v-if="candidateText(field.candidates)">可选：{{ candidateText(field.candidates) }}</small>
              <small class="form-hints__fill">{{ fillHintText(field) }}</small>
            </li>
          </ul>
        </n-collapse-item>

        <n-collapse-item v-if="blockingIssues.length" name="issues" :title="`需要处理（${blockingIssues.length}）`">
          <ul class="form-hints__list">
            <li v-for="issue in blockingIssues" :key="`${issue.code}-${issue.path || ''}-${issue.message}`">
              {{ issue.message }}
              <small v-if="issueFieldText(issue)">相关字段：{{ issueFieldText(issue) }}</small>
            </li>
          </ul>
        </n-collapse-item>

        <n-collapse-item v-if="branchPatches.length" name="patches" :title="`已按当前路径调整（${branchPatches.length}）`">
          <ul class="form-hints__list">
            <li v-for="patch in branchPatches" :key="`${patch.branchKey}-${patch.path}`">
              {{ patchText(patch) }}
            </li>
          </ul>
        </n-collapse-item>

        <n-collapse-item v-if="otherFields.length" name="other" :title="`其他条件字段（${otherFields.length}）`">
          <ul class="form-hints__list">
            <li v-for="field in otherFields" :key="field.path">
              <strong>{{ fieldLabel(field) }}</strong> {{ fieldValueText(field.current) }}
              <small class="form-hints__fill">{{ fillHintText(field) }}</small>
            </li>
          </ul>
        </n-collapse-item>
      </n-collapse>
    </n-card>
  </div>
</template>

<style scoped>
.form-hints__view {
  display: grid;
  gap: 4px;
  padding-bottom: 8px;
  margin-bottom: 8px;
  border-bottom: 1px solid var(--n-border-color, #eee);
}

.form-hints__view-label {
  font-weight: 600;
}

.form-hints__view small {
  opacity: 0.75;
}

.form-hints__fill {
  display: block;
  margin-top: 2px;
  opacity: 0.75;
}

.form-hints {
  position: absolute;
  top: 8px;
  right: 8px;
  z-index: 20;
  display: flex;
  align-items: flex-start;
  gap: 6px;
  max-height: calc(100% - 16px);
}
.form-hints__toggle {
  flex: 0 0 auto;
}
.form-hints__card {
  width: min(300px, 42vw);
  max-height: calc(100% - 24px);
  overflow: auto;
}
.form-hints__list {
  display: grid;
  gap: 6px;
  margin: 0;
  padding-left: 16px;
  font-size: 12px;
}
.form-hints__field-head {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}
.form-hints code {
  padding: 0 3px;
  font-size: 11px;
  word-break: break-all;
}
.form-hints__field-head strong {
  font-size: 12px;
}
.form-hints small {
  display: block;
  opacity: .7;
  font-size: 11px;
}
</style>
