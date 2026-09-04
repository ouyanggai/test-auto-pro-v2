<script setup lang="ts">
import { NBadge, NButton, NCard, NCollapse, NCollapseItem, NEmpty, NTag } from 'naive-ui'
import { computed, ref } from 'vue'

import type { HistoryDataIssue } from '../history-replay/types'
import type { PathConfigurationBranchPatch, PathConfigKeyField } from './types'

const props = defineProps<{
  keyFields: PathConfigKeyField[]
  issues: HistoryDataIssue[]
  branchPatches: PathConfigurationBranchPatch[]
}>()

const open = ref(true)

const decisiveFields = computed(() => props.keyFields.filter(field => field.decisive))
const otherFields = computed(() => props.keyFields.filter(field => !field.decisive))
const attentionCount = computed(() => props.issues.length + decisiveFields.value.length)
const expandedNames = computed(() => {
  const names: string[] = []
  if (decisiveFields.value.length) names.push('key')
  if (props.issues.length) names.push('issues')
  return names.length ? names : ['key']
})
const empty = computed(() => !decisiveFields.value.length && !props.issues.length
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
            </li>
          </ul>
        </n-collapse-item>

        <n-collapse-item v-if="issues.length" name="issues" :title="`需要处理（${issues.length}）`">
          <ul class="form-hints__list">
            <li v-for="issue in issues" :key="`${issue.code}-${issue.path || ''}-${issue.message}`">
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
            </li>
          </ul>
        </n-collapse-item>
      </n-collapse>
    </n-card>
  </div>
</template>

<style scoped>
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
