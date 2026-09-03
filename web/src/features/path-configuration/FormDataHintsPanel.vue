<script setup lang="ts">
import { NBadge, NButton, NTag, useThemeVars } from 'naive-ui'
import { computed, ref } from 'vue'

import type { HistoryDataIssue } from '../history-replay/types'
import type { PathConfigurationBranchPatch, PathConfigKeyField } from './types'

const props = defineProps<{
  keyFields: PathConfigKeyField[]
  issues: HistoryDataIssue[]
  branchPatches: PathConfigurationBranchPatch[]
}>()

const themeVars = useThemeVars()
const open = ref(true)

const decisiveFields = computed(() => props.keyFields.filter(field => field.decisive))
const otherFields = computed(() => props.keyFields.filter(field => !field.decisive))
const attentionCount = computed(() => props.issues.length + decisiveFields.value.length)
const panelStyle = computed(() => ({
  '--hints-card-color': themeVars.value.cardColor,
  '--hints-border-color': themeVars.value.borderColor,
  '--hints-text-color': themeVars.value.textColor1,
  '--hints-secondary-color': themeVars.value.textColor3,
}))

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
</script>

<template>
  <aside class="form-hints" :class="{ 'form-hints--collapsed': !open }" :style="panelStyle" data-testid="form-data-hints">
    <n-button
      class="form-hints__toggle"
      quaternary
      size="small"
      :aria-expanded="open"
      :title="open ? '收起提示' : '展开提示'"
      @click="open = !open"
    >
      <n-badge :value="attentionCount" :max="99" :show="!open && attentionCount > 0">
        <span aria-hidden="true">{{ open ? '›' : '‹' }}</span>
      </n-badge>
      <span class="form-hints__toggle-text">{{ open ? '收起' : '提示' }}</span>
    </n-button>

    <div v-if="open" class="form-hints__body">
      <section v-if="decisiveFields.length" class="form-hints__block">
        <h4>决定当前路径的字段</h4>
        <ul>
          <li v-for="field in decisiveFields" :key="field.path">
            <div class="form-hints__field-head">
              <code>{{ field.path }}</code>
              <n-tag size="tiny" :bordered="false" type="info">{{ fieldValueText(field.current) }}</n-tag>
            </div>
            <small v-if="candidateText(field.candidates)">可选：{{ candidateText(field.candidates) }}</small>
          </li>
        </ul>
      </section>

      <section v-if="issues.length" class="form-hints__block">
        <h4>需要处理</h4>
        <ul>
          <li v-for="issue in issues" :key="`${issue.code}-${issue.path || ''}-${issue.message}`">
            {{ issue.message }}<template v-if="issue.path"><code>{{ issue.path }}</code></template>
          </li>
        </ul>
      </section>

      <section v-if="branchPatches.length" class="form-hints__block">
        <h4>已按当前路径调整</h4>
        <ul>
          <li v-for="patch in branchPatches" :key="`${patch.branchKey}-${patch.path}`">
            <code>{{ patch.path }}</code> {{ patch.reason }}
          </li>
        </ul>
      </section>

      <section v-if="otherFields.length" class="form-hints__block">
        <h4>其他条件字段</h4>
        <ul>
          <li v-for="field in otherFields" :key="field.path">
            <code>{{ field.path }}</code> {{ fieldValueText(field.current) }}
          </li>
        </ul>
      </section>

      <p v-if="!decisiveFields.length && !issues.length && !branchPatches.length && !otherFields.length" class="form-hints__empty">
        当前路径没有需要优先核对的字段。
      </p>
    </div>
  </aside>
</template>

<style scoped>
.form-hints {
  position: absolute;
  top: 12px;
  right: 12px;
  z-index: 2;
  display: flex;
  align-items: flex-start;
  gap: 4px;
  width: min(300px, 40%);
  max-height: calc(100% - 24px);
  padding: 8px 10px;
  overflow: hidden;
  color: var(--hints-text-color);
  background: var(--hints-card-color);
  border: 1px solid var(--hints-border-color);
  border-radius: 6px;
  box-shadow: 0 2px 10px rgb(0 0 0 / 6%);
}
.form-hints--collapsed {
  width: auto;
  padding: 4px;
}
.form-hints__toggle {
  flex: 0 0 auto;
  padding: 0 4px;
}
.form-hints__toggle-text {
  margin-left: 2px;
  font-size: 12px;
}
.form-hints__body {
  display: grid;
  gap: 10px;
  min-width: 0;
  max-height: calc(100% - 8px);
  overflow: auto;
}
.form-hints__block h4 {
  margin: 0 0 4px;
  font-size: 12px;
  font-weight: 600;
}
.form-hints__block ul {
  display: grid;
  gap: 5px;
  margin: 0;
  padding-left: 14px;
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
  background: color-mix(in srgb, var(--hints-border-color) 40%, transparent);
  border-radius: 3px;
}
.form-hints small,
.form-hints__empty {
  display: block;
  color: var(--hints-secondary-color);
  font-size: 11px;
}
</style>
