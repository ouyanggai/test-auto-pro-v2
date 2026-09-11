<script setup lang="ts">
import { NButton, NEmpty, NPopconfirm, NResult, NSpin, NTag, useThemeVars } from 'naive-ui'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { deleteRun, fetchRunPaths, formatTime, RunApiError } from '../features/runs/api'
import type { RunPathProgress, RunPathsView } from '../features/runs/api'

// RunPathsView 是二级路径页：一次运行实际参与的执行路径。
// 每条路径显示名称、当前状态、当前节点、已完成步骤/总步骤与准确进度；
// 进度来自已落库步骤与本次运行冻结的总步骤，完成才是 100%，失败或停止保留实际进度。
const route = useRoute()
const router = useRouter()
const themeVars = useThemeVars()
const runId = String(route.params.runId || '')

const view = ref<RunPathsView | null>(null)
const loading = ref(true)
const errorText = ref('')
const loadFailure = ref<RunApiError | null>(null)
const viewNotFound = computed(() => loadFailure.value?.status === 404)

// deleting 表示删除请求在途：按钮忙碌，重复点击不会发出第二个删除请求。
const deleting = ref(false)

// loadPaths 读取本次运行的路径进度；轮询只在运行未结束时继续。
async function loadPaths(): Promise<void> {
  if (!runId) {
    errorText.value = '运行标识缺失，无法打开运行记录。'
    loading.value = false
    return
  }
  loading.value = !view.value
  loadFailure.value = null
  try {
    view.value = await fetchRunPaths(runId)
    errorText.value = ''
    schedulePoll()
  } catch (error) {
    loadFailure.value = error instanceof RunApiError ? error : null
    errorText.value = error instanceof RunApiError ? error.message : '暂时无法读取运行记录，请重试'
  } finally {
    loading.value = false
  }
}

// 轮询：运行未整体结束时按固定间隔刷新进度与状态。
let pollTimer: number | null = null
function schedulePoll(): void {
  if (pollTimer !== null) {
    window.clearTimeout(pollTimer)
    pollTimer = null
  }
  if (!view.value) return
  const closed = ['已完成', '失败', '已停止', '已取消'].includes(view.value.runStatusName)
  if (closed) return
  pollTimer = window.setTimeout(async () => {
    try {
      view.value = await fetchRunPaths(runId)
    } catch {
      // 单次轮询失败不打断页面：下一次轮询会继续。
    }
    schedulePoll()
  }, 2000)
}

// openPanel 进入该路径的运行面板（三层导航的第三层）。
function openPanel(path: RunPathProgress): void {
  router.push(`/runs/${runId}/paths/${path.pathRunId}`)
}

// removeRun 删除整次运行：运行中的记录必须先停止再删除（后端守卫），删除后返回运行记录列表。
async function removeRun(): Promise<void> {
  if (deleting.value) return
  deleting.value = true
  errorText.value = ''
  try {
    await deleteRun(runId)
    void router.push('/runs')
  } catch (error) {
    errorText.value = error instanceof RunApiError ? error.message : '删除失败，请重试'
  } finally {
    deleting.value = false
  }
}

// deleteHint 写清删除的影响范围：计划、运行号、路径数量与边界。
const deleteHint = computed(() => {
  if (!view.value) return ''
  return `删除计划「${view.value.planName}」的运行 #${view.value.runNo}：共 ${view.value.paths.length} 条执行路径的记录、步骤与日志引用会一并删除；目标平台上的实例和业务数据不受影响。删除后无法恢复。`
})

// progressText 把进度写成中文事实；分母缺失（历史运行）时如实显示「—」。
function progressText(path: RunPathProgress): string {
  if (!path.totalSteps) return '进度：—（这次运行没有可用的步骤总数）'
  return `已完成 ${path.doneSteps} / ${path.totalSteps} 步`
}

// statusTagType 让状态标签的颜色与语义一致；颜色之外始终有中文文字。
function statusTagType(statusName: string): 'default' | 'info' | 'success' | 'warning' | 'error' {
  switch (statusName) {
    case '已完成': return 'success'
    case '失败': return 'error'
    case '结果待确认': return 'warning'
    case '运行中':
    case '核验中': return 'info'
    default: return 'default'
  }
}

onMounted(() => { void loadPaths() })
onBeforeUnmount(() => {
  if (pollTimer !== null) window.clearTimeout(pollTimer)
})
</script>

<template>
  <section class="run-paths">
    <div v-if="loading" class="run-paths__loading"><NSpin size="small" /><span>正在读取运行记录……</span></div>
    <div v-else-if="!view" class="run-paths__result">
      <NResult
        :status="viewNotFound ? '404' : 'error'"
        size="small"
        :title="viewNotFound ? '未找到运行记录' : '运行记录读取失败'"
        :description="errorText || '未找到该运行记录。'"
        role="alert"
      />
      <div class="run-paths__result-actions">
        <NButton v-if="runId && (!loadFailure || loadFailure.retryable)" type="primary" secondary @click="loadPaths">重试</NButton>
        <NButton @click="router.push('/runs')">返回运行记录</NButton>
      </div>
    </div>

    <template v-else>
      <header class="run-paths__header">
        <div class="run-paths__identity">
          <NButton quaternary circle size="small" aria-label="返回运行记录列表" title="返回运行记录列表" @click="router.push('/runs')">←</NButton>
          <h2 class="run-paths__title">运行 #{{ view.runNo }}</h2>
          <span class="run-paths__plan">{{ view.planName }}</span>
          <NTag size="small" :bordered="false" type="info">{{ view.modeName }}模式</NTag>
          <NTag size="small" :bordered="false" :type="statusTagType(view.runStatusName)">{{ view.runStatusName }}</NTag>
          <NTag v-if="view.resultName" size="small" :bordered="false">{{ view.resultName }}</NTag>
        </div>
        <div class="run-paths__meta">
          <span>{{ view.scheduleName }} · {{ view.concurrencyLabel }}</span>
          <span>开始于 {{ formatTime(view.startedAt) }}</span>
          <span v-if="view.finishedAt">结束于 {{ formatTime(view.finishedAt) }}</span>
          <NPopconfirm @positive-click="removeRun">
            <template #trigger>
              <NButton size="small" type="error" ghost :loading="deleting">删除本次运行</NButton>
            </template>
            {{ deleteHint }}
          </NPopconfirm>
        </div>
      </header>

      <p class="run-paths__hint">下面是这次运行实际执行的路径，不是计划的全部路径。点一条路径可进入运行面板查看画布与详情。</p>

      <p v-if="errorText" class="run-paths__error" role="alert">{{ errorText }}</p>

      <NEmpty
        v-if="view.paths.length === 0"
        description="这次运行没有路径记录。"
      />

      <ul v-else class="run-paths__list">
        <li v-for="path in view.paths" :key="path.pathRunId">
          <button type="button" class="run-paths__card" @click="openPanel(path)">
            <div class="run-paths__card-head">
              <span class="run-paths__card-name">{{ path.pathName }}</span>
              <NTag size="small" :bordered="false" :type="statusTagType(path.statusName)">{{ path.statusName }}</NTag>
              <NTag v-if="path.resultName" size="small" :bordered="false">{{ path.resultName }}</NTag>
              <NTag v-if="path.failureClassName" size="small" :bordered="false" type="error">{{ path.failureClassName }}</NTag>
            </div>
            <div class="run-paths__card-body">
              <span>当前节点：{{ path.currentNodeName || '尚未开始' }}</span>
              <span>{{ progressText(path) }}</span>
              <span>开始于 {{ formatTime(path.startedAt) }}</span>
              <span v-if="path.finishedAt">结束于 {{ formatTime(path.finishedAt) }}</span>
            </div>
            <div v-if="path.totalSteps" class="run-paths__progress" role="img" :aria-label="`进度 ${path.progressPercent}%`">
              <!-- 运行未结束时进度条叠加流光动画，表示任务正在推进；终态后动画停止。 -->
              <div
                class="run-paths__progress-bar"
                :class="{ 'run-paths__progress-bar--active': !path.finishedAt }"
                :style="{ width: `${path.progressPercent}%` }"
              />
            </div>
          </button>
        </li>
      </ul>
    </template>
  </section>
</template>

<style scoped>
.run-paths {
  display: grid;
  gap: 14px;
  align-content: start;
  padding: 20px 24px;
}

.run-paths__loading {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 20px 0;
}

.run-paths__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.run-paths__identity {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.run-paths__title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  white-space: nowrap;
}

.run-paths__plan {
  overflow: hidden;
  max-width: 32vw;
  opacity: 0.8;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.run-paths__meta {
  display: flex;
  align-items: center;
  gap: 14px;
  flex-wrap: wrap;
  font-size: 13px;
  opacity: 0.85;
}

.run-paths__hint {
  margin: 0;
  font-size: 13px;
  opacity: 0.75;
}

.run-paths__error { margin: 0; color: var(--error-color, #d03050); }

.run-paths__result-actions {
  display: flex;
  justify-content: center;
  gap: 8px;
}

.run-paths__result {
  text-align: center;
}

.run-paths__list {
  display: grid;
  gap: 10px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.run-paths__card {
  display: grid;
  gap: 8px;
  width: 100%;
  padding: 12px 14px;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
  background: transparent;
  border: 1px solid v-bind('themeVars.dividerColor');
  border-radius: 8px;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.run-paths__card:hover {
  border-color: v-bind('themeVars.primaryColor');
  box-shadow: 0 2px 12px rgb(0 0 0 / 8%);
}

.run-paths__card:focus-visible {
  outline: 2px solid v-bind('themeVars.primaryColor');
  outline-offset: 2px;
}

.run-paths__card-head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.run-paths__card-name {
  font-weight: 600;
}

.run-paths__card-body {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
  font-size: 13px;
  opacity: 0.85;
}

.run-paths__progress {
  height: 6px;
  overflow: hidden;
  background: color-mix(in srgb, v-bind('themeVars.dividerColor') 55%, transparent);
  border-radius: 3px;
}

.run-paths__progress-bar {
  height: 100%;
  background: v-bind('themeVars.primaryColor');
  border-radius: 3px;
  transition: width 0.5s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
}

/* 运行中流光：一条高亮斜纹在进度条内循环移动，直观表达「正在运行」。 */
.run-paths__progress-bar--active::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: linear-gradient(
    100deg,
    transparent 20%,
    rgba(255, 255, 255, 0.45) 50%,
    transparent 80%
  );
  background-size: 200% 100%;
  animation: run-paths-shimmer 1.6s linear infinite;
}

@keyframes run-paths-shimmer {
  from { background-position: 200% 0; }
  to { background-position: -200% 0; }
}
</style>
