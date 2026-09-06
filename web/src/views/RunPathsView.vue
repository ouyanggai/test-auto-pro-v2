<script setup lang="ts">
import { NButton, NEmpty, NPopconfirm, NSpin, NTag, useThemeVars } from 'naive-ui'
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
const loading = ref(false)
const errorText = ref('')

// deleting 表示删除请求在途：按钮忙碌，重复点击不会发出第二个删除请求。
const deleting = ref(false)

// loadPaths 读取本次运行的路径进度；轮询只在运行未结束时继续。
async function loadPaths(): Promise<void> {
  if (!runId) {
    errorText.value = '运行标识缺失，无法打开运行记录。'
    return
  }
  loading.value = !view.value
  try {
    view.value = await fetchRunPaths(runId)
    errorText.value = ''
    schedulePoll()
  } catch (error) {
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
    <NEmpty v-else-if="!view" :description="errorText || '未找到该运行记录。'" />

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
              <div class="run-paths__progress-bar" :style="{ width: `${path.progressPercent}%` }" />
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
}
</style>
