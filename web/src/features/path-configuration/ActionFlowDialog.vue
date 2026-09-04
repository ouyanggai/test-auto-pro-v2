<script setup lang="ts">
import { NAlert, NButton, NEmpty, NModal, NSpin, NTag, useThemeVars } from 'naive-ui'
import { computed } from 'vue'

import { pathActionFlowParameters, pathActionFlowSegments, pathActionFlowStepName } from './logic'
import type { PathActionFlowLabels } from './logic'
import type { PathActionConfigurationIssue, PathActionStepSource, PathCompiledActionStep } from './types'

const props = defineProps<{
  show: boolean
  steps: PathCompiledActionStep[]
  issues: PathActionConfigurationIssue[]
  labels: PathActionFlowLabels
  currentNodeKey: string
  loading: boolean
  error: string
}>()
const emit = defineEmits<{ 'update:show': [value: boolean]; reload: [] }>()

const themeVars = useThemeVars()

// 三类步骤必须视觉可分：用户配置的动作、系统为了让路径继续而插入的恢复步骤、目标引擎自己的导航。
const SOURCE_LABELS: Record<PathActionStepSource, string> = {
  user: '用户动作',
  system_recovery: '系统恢复',
  system_navigation: '系统导航',
}
const SOURCE_TAGS: Record<PathActionStepSource, 'success' | 'warning' | 'info'> = {
  user: 'success',
  system_recovery: 'warning',
  system_navigation: 'info',
}

const segments = computed(() => pathActionFlowSegments(props.steps, props.labels, props.currentNodeKey))
const userStepCount = computed(() => props.steps.filter(step => step.source === 'user').length)
const systemStepCount = computed(() => props.steps.length - userStepCount.value)
const blockingIssues = computed(() => props.issues.filter(issue => issue.blocking))
const advisoryIssues = computed(() => props.issues.filter(issue => !issue.blocking))
// 宽度和颜色写在行内样式：NModal 的卡片 teleport 到 body 渲染，scoped 样式选不中它。
const dialogStyle = computed(() => ({
  width: '720px',
  maxWidth: 'calc(100vw - 48px)',
  '--flow-border-color': themeVars.value.borderColor,
  '--flow-text-color': themeVars.value.textColor2,
  '--flow-secondary-text-color': themeVars.value.textColor3,
  '--flow-current-color': themeVars.value.primaryColor,
  '--flow-current-background': themeVars.value.actionColor,
  '--flow-user-color': themeVars.value.successColor,
  '--flow-recovery-color': themeVars.value.warningColor,
  '--flow-navigation-color': themeVars.value.infoColor,
}))

// sourceLabel 与 sourceTag 保证系统插入的步骤不会被显示成用户配置的动作。
function sourceLabel(source: PathActionStepSource) {
  return SOURCE_LABELS[source] ?? source
}
function sourceTag(source: PathActionStepSource) {
  return SOURCE_TAGS[source] ?? 'default'
}
// stepName 显示目录里的中文动作名，动作键只在目录缺名时兜底。
function stepName(step: PathCompiledActionStep) {
  return pathActionFlowStepName(step, props.labels)
}
// stepDetails 把执行器要遵守的门禁收进折叠区，流程主干只留顺序和动作名。
function stepDetails(step: PathCompiledActionStep) {
  const parameters = pathActionFlowParameters(step)
  return [
    { name: '前置事实', value: step.precondition },
    { name: '演员策略', value: step.actorPolicy ?? '' },
    { name: '参数', value: parameters.join('；') },
    { name: '失败停止条件', value: step.stopOnFailure },
    { name: '恢复策略', value: step.recoveryPolicy },
  ].filter(row => Boolean(row.value))
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    title="动作执行流程"
    data-testid="action-flow-dialog"
    :style="dialogStyle"
    :bordered="false"
    size="small"
    @update:show="value => emit('update:show', value)"
  >
    <n-spin :show="loading">
      <div class="action-flow">
        <p class="action-flow__note">
          服务端按当前路径、处理人员和动作门禁编译出的只读执行顺序，只反映已保存的配置，不代表已经执行。
          <span v-if="steps.length">共 {{ steps.length }} 步：用户动作 {{ userStepCount }}，系统插入 {{ systemStepCount }}。</span>
        </p>
        <n-alert v-if="error" type="error" :show-icon="false">{{ error }}</n-alert>
        <n-alert v-for="issue in blockingIssues" :key="`blocking-${issue.index}-${issue.code}`" type="error" :show-icon="false">
          第 {{ issue.index + 1 }} 条动作：{{ issue.message }}
        </n-alert>
        <n-alert v-for="issue in advisoryIssues" :key="`advisory-${issue.index}-${issue.code}`" type="warning" :show-icon="false">
          第 {{ issue.index + 1 }} 条动作：{{ issue.message }}
        </n-alert>
        <ol v-if="segments.length" class="action-flow__chain" data-testid="action-flow-chain">
          <li v-for="(segment, segmentIndex) in segments" :key="`${segment.nodeKey}-${segment.steps[0].sequence}`" class="action-flow__segment">
            <div :class="['action-flow__group', segment.current ? 'action-flow__group--current' : '']">
              <header class="action-flow__group-head">
                <strong>{{ segment.title }}</strong>
                <n-tag v-if="segment.current" size="tiny" type="primary">当前节点</n-tag>
                <span class="action-flow__group-count">{{ segment.steps.length }} 步</span>
              </header>
              <div v-for="(step, index) in segment.steps" :key="step.sequence" class="action-flow__step">
                <div :class="['action-flow__box', `action-flow__box--${step.source}`]">
                  <div class="action-flow__box-head">
                    <span class="action-flow__sequence">{{ step.sequence }}</span>
                    <strong>{{ stepName(step) }}</strong>
                    <n-tag size="tiny" :type="sourceTag(step.source)">{{ sourceLabel(step.source) }}</n-tag>
                    <n-tag v-if="step.reloadRequired" size="tiny">执行前重读目标事实</n-tag>
                  </div>
                  <!-- 这里显示服务端给出的中文预期效果，不显示节点键和动作键这类内部标识。 -->
                  <p v-if="step.expectedEffect" class="action-flow__effect">{{ step.expectedEffect }}</p>
                  <details v-if="stepDetails(step).length" class="action-flow__detail">
                    <summary>步骤细节</summary>
                    <p v-for="row in stepDetails(step)" :key="row.name">
                      <span class="action-flow__detail-name">{{ row.name }}</span>{{ row.value }}
                    </p>
                  </details>
                </div>
                <span v-if="index < segment.steps.length - 1" class="action-flow__arrow" aria-hidden="true">↓</span>
              </div>
            </div>
            <span v-if="segmentIndex < segments.length - 1" class="action-flow__arrow" aria-hidden="true">↓</span>
          </li>
        </ol>
        <n-empty v-else-if="!loading && !error" size="small" description="当前还没有编译出可执行步骤，请先配置节点动作并保存" />
      </div>
    </n-spin>
    <template #footer>
      <div class="action-flow__footer">
        <n-button size="small" :loading="loading" @click="emit('reload')">重新读取</n-button>
        <n-button size="small" @click="emit('update:show', false)">关闭</n-button>
      </div>
    </template>
  </n-modal>
</template>

<style scoped>
.action-flow {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-height: 62vh;
  overflow-y: auto;
  color: var(--flow-text-color);
}

.action-flow__note {
  margin: 0;
  line-height: 1.6;
  color: var(--flow-secondary-text-color);
}

.action-flow__chain,
.action-flow__segment {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  margin: 0;
  padding: 0;
  list-style: none;
}

.action-flow__group {
  border: 1px dashed var(--flow-border-color);
  border-radius: 6px;
  padding: 8px 10px 10px;
  display: flex;
  flex-direction: column;
}

.action-flow__group--current {
  border-style: solid;
  border-color: var(--flow-current-color);
  background: var(--flow-current-background);
}

.action-flow__group-head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  padding-bottom: 6px;
}

.action-flow__group-count {
  color: var(--flow-secondary-text-color);
}

.action-flow__step {
  display: flex;
  flex-direction: column;
  align-items: stretch;
}

.action-flow__box {
  border: 1px solid var(--flow-border-color);
  border-left-width: 3px;
  border-radius: 4px;
  padding: 8px 10px;
}

.action-flow__box--user {
  border-left-color: var(--flow-user-color);
}

.action-flow__box--system_recovery {
  border-left-color: var(--flow-recovery-color);
}

.action-flow__box--system_navigation {
  border-left-color: var(--flow-navigation-color);
}

.action-flow__box-head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.action-flow__sequence {
  min-width: 22px;
  height: 22px;
  border: 1px solid var(--flow-border-color);
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--flow-secondary-text-color);
}

.action-flow__effect {
  margin: 4px 0 0;
  color: var(--flow-secondary-text-color);
}

.action-flow__detail {
  margin-top: 6px;
  color: var(--flow-secondary-text-color);
}

.action-flow__detail summary {
  cursor: pointer;
}

.action-flow__detail p {
  margin: 4px 0 0;
  line-height: 1.6;
}

.action-flow__detail-name {
  display: inline-block;
  min-width: 84px;
  color: var(--flow-text-color);
}

.action-flow__arrow {
  align-self: center;
  color: var(--flow-secondary-text-color);
  line-height: 1.2;
  padding: 2px 0;
}

.action-flow__footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
