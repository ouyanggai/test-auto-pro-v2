<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { NAlert, NButton, NCard, NCode, NDescriptions, NDescriptionsItem, NSpin, NTag, useMessage } from 'naive-ui'
import { RouterLink } from 'vue-router'

import {
  createRuntimeSyncJob,
  fetchLatestRuntimeSyncJob,
  fetchRuntimeSource,
  fetchRuntimeSyncJob,
  fetchRuntimeSyncLog,
  type RuntimeSourceState,
  type RuntimeSyncJob,
  type RuntimeSyncLog,
} from '../features/form-runtime-maintenance/api'

const message = useMessage()
const source = ref<RuntimeSourceState | null>(null)
const job = ref<RuntimeSyncJob | null>(null)
const log = ref<RuntimeSyncLog | null>(null)
const loading = ref(true)
const creating = ref(false)
const errorMessage = ref('')
let pollTimer: number | undefined
let requestVersion = 0

const active = computed(() => job.value?.status === 'PENDING' || job.value?.status === 'RUNNING')
const stageName = computed(() => ({
  QUEUED: '排队', INSPECT: '来源复核', SYNC: '同步原样区', SYNC_CHECK: '同步校验', BUILD: '构建候选',
  RESTART: '切换候选', VERIFY: '健康核验', COMPLETED: '已完成',
}[job.value?.stage || 'QUEUED']))
const statusType = computed(() => job.value?.status === 'SUCCEEDED' ? 'success' : job.value?.status === 'FAILED' ? 'error' : 'warning')

// loadInitial 并发读取固定来源和最近任务；没有历史任务不影响来源展示。
async function loadInitial() {
  const version = ++requestVersion
  loading.value = true
  errorMessage.value = ''
  try {
    source.value = await fetchRuntimeSource()
    try { job.value = await fetchLatestRuntimeSyncJob() }
    catch { job.value = null }
    if (version !== requestVersion) return
    if (job.value) await refreshJob(job.value.id, version)
  }
  catch (error) {
    if (version === requestVersion) errorMessage.value = error instanceof Error ? error.message : '维护状态读取失败'
  }
  finally {
    if (version === requestVersion) loading.value = false
  }
}

// refreshJob 同一轮读取任务和日志，旧轮询结果不能覆盖新任务。
async function refreshJob(id: number, version = requestVersion) {
  const [nextJob, nextLog] = await Promise.all([
    fetchRuntimeSyncJob(id),
    fetchRuntimeSyncLog(id).catch(() => null),
  ])
  if (version !== requestVersion || id !== nextJob.id) return
  job.value = nextJob
  log.value = nextLog
  schedulePoll()
}

// schedulePoll 只在任务活跃时轮询，终态不会继续占用请求。
function schedulePoll() {
  if (pollTimer) window.clearTimeout(pollTimer)
  pollTimer = undefined
  if (!active.value || !job.value) return
  const id = job.value.id
  const version = requestVersion
  pollTimer = window.setTimeout(() => refreshJob(id, version).catch((error) => {
    if (version === requestVersion) errorMessage.value = error instanceof Error ? error.message : '任务状态读取失败'
  }), 1000)
}

// startSync 只调用固定一键任务，界面不暴露任意路径、分支或命令输入。
async function startSync() {
  creating.value = true
  errorMessage.value = ''
  const version = ++requestVersion
  try {
    job.value = await createRuntimeSyncJob()
    log.value = null
    message.success('同步任务已创建')
    await refreshJob(job.value.id, version)
  }
  catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '同步任务创建失败'
  }
  finally { creating.value = false }
}

onMounted(loadInitial)
onBeforeUnmount(() => {
  requestVersion++
  if (pollTimer) window.clearTimeout(pollTimer)
})
</script>

<template>
  <section class="runtime-maintenance">
    <header class="runtime-maintenance__header">
      <div>
        <router-link to="/settings">返回系统设置</router-link>
        <h1>表单运行时维护</h1>
        <p>只从项目固定参考仓库同步，候选验证通过后才替换当前可用版本。</p>
      </div>
      <n-button type="primary" :loading="creating" :disabled="loading || active || source?.dirty" @click="startSync">一键同步并重启</n-button>
    </header>

    <n-alert v-if="errorMessage" type="error" :show-icon="false">{{ errorMessage }}</n-alert>
    <n-spin :show="loading">
      <div class="runtime-maintenance__grid">
        <n-card title="固定来源" size="small">
          <n-descriptions v-if="source" :column="1" label-placement="left" bordered size="small">
            <n-descriptions-item label="仓库">{{ source.repository }}</n-descriptions-item>
            <n-descriptions-item label="分支">{{ source.branch }}</n-descriptions-item>
            <n-descriptions-item label="HEAD"><code>{{ source.head }}</code></n-descriptions-item>
            <n-descriptions-item label="工作树"><n-tag :type="source.dirty ? 'error' : 'success'" size="small">{{ source.dirty ? '有修改，禁止同步' : '干净' }}</n-tag></n-descriptions-item>
            <n-descriptions-item label="检查时间">{{ source.inspectedAt }}</n-descriptions-item>
          </n-descriptions>
        </n-card>

        <n-card title="最近任务" size="small">
          <div v-if="job" class="runtime-maintenance__job">
            <div><n-tag :type="statusType" size="small">{{ job.status }}</n-tag><strong>{{ stageName }}</strong><span>第 {{ job.attemptCount }} 次领取</span></div>
            <p v-if="job.recoveryMessage">恢复结果：{{ job.recoveryMessage }}</p>
            <p v-if="job.failureReason" class="runtime-maintenance__failure">{{ job.failureReason }}</p>
          </div>
          <p v-else>尚无维护任务。</p>
        </n-card>
      </div>

      <n-card title="在线日志" size="small">
        <p v-if="log?.truncated">日志较长，以下只显示最新部分。</p>
        <n-code v-if="log?.content" :code="log.content" language="text" word-wrap />
        <p v-else>任务开始后在此显示阶段与构建日志。</p>
      </n-card>
    </n-spin>
  </section>
</template>

<style scoped>
.runtime-maintenance { display: grid; gap: 16px; padding: 24px; }
.runtime-maintenance__header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.runtime-maintenance__header h1 { margin: 6px 0 4px; }
.runtime-maintenance__header p { margin: 0; color: var(--n-text-color-3); }
.runtime-maintenance__grid { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 16px; margin-bottom: 16px; }
.runtime-maintenance__job > div { display: flex; align-items: center; gap: 10px; }
.runtime-maintenance__failure { color: var(--n-error-color); }
code { word-break: break-all; }
@media (max-width: 760px) { .runtime-maintenance__grid { grid-template-columns: 1fr; } .runtime-maintenance__header { flex-direction: column; } }
</style>
