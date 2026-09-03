<script setup lang="ts">
import { NAlert, NEmpty, NTag } from 'naive-ui'
import { computed } from 'vue'
import type { PathActionConfigurationIssue, PathActionStepSource, PathCompiledActionStep } from './types'

const props = defineProps<{ steps: PathCompiledActionStep[]; issues: PathActionConfigurationIssue[]; loading: boolean; error: string }>()

const SOURCE_LABELS: Record<PathActionStepSource, string> = { user: '用户动作', system_recovery: '系统恢复', system_navigation: '系统导航' }
const SOURCE_TAGS: Record<PathActionStepSource, 'success' | 'warning' | 'info'> = { user: 'success', system_recovery: 'warning', system_navigation: 'info' }

const orderedSteps = computed(() => [...props.steps].sort((left, right) => left.sequence - right.sequence))
const userStepCount = computed(() => props.steps.filter(step => step.source === 'user').length)
const systemStepCount = computed(() => props.steps.length - userStepCount.value)
const blockingIssues = computed(() => props.issues.filter(issue => issue.blocking))
const advisoryIssues = computed(() => props.issues.filter(issue => !issue.blocking))

// sourceLabel 用中文区分用户动作、系统恢复步骤和系统导航步骤。
function sourceLabel(source: PathActionStepSource): string { return SOURCE_LABELS[source] ?? source }
// sourceTag 给三类步骤不同的视觉分组，系统步骤不能显示为用户配置动作。
function sourceTag(source: PathActionStepSource) { return SOURCE_TAGS[source] ?? 'default' }
// parameterEntries 只展示服务端编译出的参数，浏览器不提交任何步骤正文。
function parameterEntries(step: PathCompiledActionStep) { return Object.entries(step.parameters ?? {}).map(([name, value]) => `${name}=${typeof value === 'object' ? JSON.stringify(value) : String(value)}`) }
</script>

<template>
  <section class="compiled-scenario">
    <header class="compiled-scenario__header">
      <h3>编译场景预览（只读）</h3>
      <span v-if="steps.length" class="compiled-scenario__count">共 {{ steps.length }} 步：用户动作 {{ userStepCount }}，系统插入 {{ systemStepCount }}</span>
    </header>
    <p class="compiled-scenario__note">以下步骤由服务端按当前路径、人员和动作门禁编译，仅用于核对，不代表已执行。</p>

    <n-alert v-if="error" type="error" :show-icon="false">{{ error }}</n-alert>
    <n-alert v-for="issue in blockingIssues" :key="`blocking-${issue.index}-${issue.code}`" type="error" :show-icon="false">第 {{ issue.index + 1 }} 条动作：{{ issue.message }}</n-alert>
    <n-alert v-for="issue in advisoryIssues" :key="`advisory-${issue.index}-${issue.code}`" type="warning" :show-icon="false">第 {{ issue.index + 1 }} 条动作：{{ issue.message }}</n-alert>

    <p v-if="loading" class="compiled-scenario__note">正在读取编译结果…</p>
    <ol v-else-if="orderedSteps.length" class="compiled-scenario__steps">
      <li v-for="step in orderedSteps" :key="step.sequence" :class="`compiled-scenario__step compiled-scenario__step--${step.source}`">
        <div class="compiled-scenario__step-head">
          <strong>第 {{ step.sequence }} 步</strong>
          <n-tag size="tiny" :type="sourceTag(step.source)">{{ sourceLabel(step.source) }}</n-tag>
          <span class="compiled-scenario__action">{{ step.action }}</span>
          <n-tag v-if="step.reloadRequired" size="tiny">重读事实屏障</n-tag>
        </div>
        <p v-if="step.actorPolicy">演员策略：{{ step.actorPolicy }}</p>
        <p v-if="parameterEntries(step).length">参数：{{ parameterEntries(step).join('；') }}</p>
        <p v-if="step.precondition">前置事实：{{ step.precondition }}</p>
        <p v-if="step.expectedEffect">预期效果：{{ step.expectedEffect }}</p>
        <p v-if="step.stopOnFailure">失败停止条件：{{ step.stopOnFailure }}</p>
        <p v-if="step.recoveryPolicy">恢复策略：{{ step.recoveryPolicy }}</p>
      </li>
    </ol>
    <n-empty v-else description="尚未编译出可核对的步骤，请先配置节点动作" />
  </section>
</template>

<style scoped>
.compiled-scenario{display:flex;flex-direction:column;gap:8px}
.compiled-scenario__header{display:flex;align-items:baseline;justify-content:space-between;gap:12px;flex-wrap:wrap}
.compiled-scenario h3{margin:0}
.compiled-scenario p{margin:0;color:#64748b}
.compiled-scenario__count{color:#475569}
.compiled-scenario__steps{list-style:none;margin:0;padding:0;display:flex;flex-direction:column;gap:8px}
.compiled-scenario__step{border:1px solid #e5e7eb;border-left-width:3px;border-radius:6px;padding:8px 10px;font-size:12px;display:flex;flex-direction:column;gap:3px}
.compiled-scenario__step--user{border-left-color:#18a058}
.compiled-scenario__step--system_recovery{border-left-color:#f0a020}
.compiled-scenario__step--system_navigation{border-left-color:#2080f0}
.compiled-scenario__step-head{display:flex;align-items:center;gap:6px;flex-wrap:wrap}
.compiled-scenario__action{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;color:#334155}
</style>
