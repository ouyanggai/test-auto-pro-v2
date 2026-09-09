<script setup lang="ts">
import { NAlert, NButton, NCheckbox, NModal, NSpin, useThemeVars } from 'naive-ui'
import { computed, ref, watch } from 'vue'

import { useRouter } from 'vue-router'

import { startRun } from '../runs/api'
import { fetchPlanRunReadiness, RunReadinessApiError } from './api'
import type { PathRunReadiness, PlanRunReadiness, RunReadinessItem } from './types'

const props = defineProps<{ show: boolean, planId: string, pathIds: string[] }>()
const emit = defineEmits<{ 'update:show': [value: boolean], locate: [pathId: string, anchor: string] }>()
const router = useRouter()
const starting = ref(false)
const startError = ref('')

// F-017：运行方式三选一（默认一步一步——最保守）。
// 选项标题说大白话，说明一行讲清行为；用户不需要理解“写请求”“断点”等内部概念。
const mode = ref<string>('single_step')
const modeOptions = [
  { value: 'single_step', title: '一步一步运行', description: '每一步都停下来，等我确认后再继续' },
  { value: 'auto', title: '自动运行', description: '自动往下执行；第一次写入数据前会停下来' },
  { value: 'manual_control', title: '手动控制', description: '先停在第一步，之后每一步都由我控制' },
]
const firstWriteBreakpoint = ref(true)

const themeVars = useThemeVars()
const readiness = ref<PlanRunReadiness | null>(null)
const loading = ref(false)
const error = ref('')
let controller: AbortController | null = null

const blockedPaths = computed<PathRunReadiness[]>(() => (readiness.value?.paths ?? []).filter(path => !path.runnable))
const runnablePaths = computed<PathRunReadiness[]>(() => (readiness.value?.paths ?? []).filter(path => path.runnable))
// S01 部分运行：默认勾选全部可运行路径，用户可以只勾选本次要运行的部分；
// 未勾选的路径不会进入运行，也不占用串并名额。
const selectedRunIds = ref<Set<string>>(new Set())
const selectedRunnable = computed<PathRunReadiness[]>(() => runnablePaths.value.filter(path => selectedRunIds.value.has(String(path.pathId))))
const canStart = computed(() => selectedRunnable.value.length > 0)

// toggleRunPath 维护本次运行勾选；勾选集合只影响启动范围，不改变运行前检查的检查范围。
function toggleRunPath(path: PathRunReadiness, included: boolean) {
  const next = new Set(selectedRunIds.value)
  const key = String(path.pathId)
  if (included) next.add(key)
  else next.delete(key)
  selectedRunIds.value = next
}

// 每次重新检查后恢复默认全选可运行路径：让"全部运行"永远是零成本默认，部分运行是一步取消。
watch(runnablePaths, (paths) => {
  selectedRunIds.value = new Set(paths.map(path => String(path.pathId)))
})
// allClear 语义收窄为"有路径可运行"：阻塞路径的存在不再阻止运行就绪子集（失败隔离 S05）。
const hasPaths = computed(() => Boolean(readiness.value) && (readiness.value?.totalCount ?? 0) > 0)
// 宽度必须写成行内样式：NModal 的卡片是 teleport 出去渲染的，scoped 样式选不中它，
// 只靠 class 设宽度会退化成撑满整屏。
const dialogStyle = computed(() => ({
  width: '720px',
  maxWidth: 'calc(100vw - 48px)',
  '--preflight-border-color': themeVars.value.borderColor,
  '--preflight-secondary-text-color': themeVars.value.textColor3,
  '--preflight-primary-color': themeVars.value.primaryColor,
}))

// runCheck 只检查本次勾选的路径：运行也只运行勾选路径，两边范围必须一致。
async function runCheck() {
  controller?.abort()
  const active = new AbortController()
  controller = active
  loading.value = true
  error.value = ''
  readiness.value = null
  try {
    const result = await fetchPlanRunReadiness(props.planId, props.pathIds, active.signal)
    if (active.signal.aborted) return
    readiness.value = result
  }
  catch (caught) {
    if (active.signal.aborted) return
    error.value = caught instanceof RunReadinessApiError ? caught.message : '暂时无法完成运行前检查，请重试'
  }
  finally {
    if (controller === active) {
      controller = null
      loading.value = false
    }
  }
}

// startSelectedRun 只启动本次勾选且可执行的路径（F-020 多路径 + S01 部分运行）：
// 串并方式来自计划配置，调度与失败隔离由后端负责；启动前服务端会再次复验运行准备结论。
// 幂等键每次启动生成一次：同一次点击的重试不会创建第二个运行。
async function startSelectedRun() {
  const targets = selectedRunnable.value
  if (targets.length === 0 || starting.value) return
  starting.value = true
  startError.value = ''
  try {
    const result = await startRun(
      props.planId,
      targets.map(path => String(path.pathId)),
      mode.value,
      [...(firstWriteBreakpoint.value ? [{ type: 'first_write' }] : [])],
      crypto.randomUUID(),
    )
    emit('update:show', false)
    router.push(`/runs/${result.runId}`)
  }
  catch (caught) {
    startError.value = caught instanceof RunReadinessApiError || caught instanceof Error ? caught.message : '启动失败，请重试'
  }
  finally {
    starting.value = false
  }
}

// locate 关闭弹窗并把定位交回页面处理，组件内部不拼路由。
function locate(pathId: string, item: RunReadinessItem) {
  emit('update:show', false)
  emit('locate', pathId, item.anchor)
}

watch(() => props.show, (open) => {
  if (open) {
    void runCheck()
    return
  }
  controller?.abort()
}, { immediate: true })
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    title="开始运行"
    class="run-preflight"
    data-testid="run-preflight-dialog"
    :style="dialogStyle"
    :bordered="false"
    size="small"
    :mask-closable="!loading"
    @update:show="value => emit('update:show', value)"
  >
    <n-spin :show="loading" class="run-preflight__body">
      <n-alert v-if="error" type="error" :show-icon="false">{{ error }}</n-alert>

      <!-- 有可运行路径：结论一行；勾选本次要运行的路径；运行方式用卡片表达；一个保护开关。 -->
      <template v-if="runnablePaths.length">
        <div
          class="run-preflight__verdict"
          :class="blockedPaths.length ? 'run-preflight__verdict--blocked' : 'run-preflight__verdict--ok'"
          data-testid="run-preflight-summary"
        >
          <span class="run-preflight__verdict-icon">{{ blockedPaths.length ? '!' : '✓' }}</span>
          <span>
            已检查 {{ readiness?.totalCount }} 条路径：{{ runnablePaths.length }} 条可以运行<template v-if="blockedPaths.length">，{{ blockedPaths.length }} 条还不能运行</template>。
          </span>
        </div>

        <p class="run-preflight__section-label">选择本次要运行的路径（{{ selectedRunnable.length }}/{{ runnablePaths.length }}）</p>
        <div class="run-preflight__paths" data-testid="run-preflight-paths">
          <n-checkbox
            v-for="path in runnablePaths"
            :key="path.pathId"
            :checked="selectedRunIds.has(String(path.pathId))"
            @update:checked="value => toggleRunPath(path, value as boolean)"
          >
            {{ path.pathName }}
          </n-checkbox>
        </div>
        <p v-if="!selectedRunnable.length" class="run-preflight__muted">还没有勾选任何路径，勾选后才能开始运行。</p>

        <template v-if="selectedRunnable.length">
          <p class="run-preflight__section-label">运行方式</p>
          <div class="run-preflight__cards" role="radiogroup" aria-label="运行方式">
            <button
              v-for="option in modeOptions"
              :key="option.value"
              type="button"
              class="run-preflight__card"
              :class="{ 'run-preflight__card--active': mode === option.value }"
              role="radio"
              :aria-checked="mode === option.value"
              @click="mode = option.value"
            >
              <span class="run-preflight__card-title">{{ option.title }}</span>
              <span class="run-preflight__card-desc">{{ option.description }}</span>
            </button>
          </div>

          <p class="run-preflight__section-label">安全保护</p>
          <n-checkbox v-model:checked="firstWriteBreakpoint" class="run-preflight__guard">
            第一次写入数据前，先停下来让我确认（建议开启）
          </n-checkbox>
          <p class="run-preflight__muted">路线和配置对不上时会自动停下，不会硬跑。</p>
        </template>
      </template>

      <!-- 有阻塞：只说哪些路径、因为什么，点击直接跳去处理。 -->
      <template v-else-if="blockedPaths.length">
        <div class="run-preflight__verdict run-preflight__verdict--blocked">
          <span class="run-preflight__verdict-icon">!</span>
          <span>有 {{ blockedPaths.length }} 条路径还不能运行</span>
        </div>
        <p class="run-preflight__lead">先处理下面的问题，处理完就能运行。点击可跳到对应位置：</p>
        <div class="run-preflight__blocks" data-testid="run-readiness-blocks">
          <button
            v-for="path in blockedPaths"
            :key="path.pathId"
            type="button"
            class="run-preflight__block"
            @click="locate(path.pathId, path.blocks[0])"
          >
            <span class="run-preflight__block-name">{{ path.pathName }}</span>
            <span class="run-preflight__block-reason">{{ path.blocks[0]?.reason }}</span>
          </button>
        </div>
      </template>

      <p v-else-if="hasPaths" class="run-preflight__muted">本次勾选的路径都没有可运行的内容。</p>
    </n-spin>

    <template #footer>
      <n-alert v-if="startError" type="error" :show-icon="false" class="run-preflight__start-error">{{ startError }}</n-alert>
      <div class="run-preflight__footer">
        <n-button size="small" quaternary @click="emit('update:show', false)">关闭</n-button>
        <!-- F-016/F-020/S01：只启动本次勾选的可执行路径，多路径按计划配置串行或并行。 -->
        <n-button
          size="small"
          type="primary"
          :loading="starting"
          :disabled="!canStart"
          @click="startSelectedRun"
        >
          {{ canStart ? '开始运行' : (blockedPaths.length ? '有问题待处理' : '请先勾选路径') }}
        </n-button>
      </div>
    </template>
  </n-modal>
</template>

<style scoped>
.run-preflight__body {
  max-height: 62vh;
  overflow-y: auto;
}

/* 结论条：图标 + 一句话，浅底色块，一眼看清能不能跑。 */
.run-preflight__verdict {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 4px;
  margin-bottom: 16px;
  font-weight: 500;
}

.run-preflight__verdict--ok {
  background: rgba(24, 160, 88, 0.08);
  color: #18a058;
}

.run-preflight__verdict--blocked {
  background: rgba(240, 160, 32, 0.1);
  color: #f0a020;
}

.run-preflight__verdict-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: currentColor;
  color: #fff;
  font-size: 12px;
  font-weight: 700;
  flex: none;
}

/* 路径勾选区：两列栅格，勾掉即不进入本次运行。 */
.run-preflight__paths {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px 12px;
  margin-bottom: 16px;
}

/* 区块标题：小号灰字，统一间距节奏。 */
.run-preflight__section-label {
  margin: 0 0 8px;
  font-size: 13px;
  color: var(--preflight-secondary-text-color);
}

/* 运行方式卡片：标题 + 说明的选中卡片，选中的用主题色描边。 */
.run-preflight__cards {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 16px;
}

.run-preflight__card {
  display: flex;
  flex-direction: column;
  gap: 2px;
  border: 1px solid var(--preflight-border-color);
  border-radius: 6px;
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
  padding: 10px 12px;
  cursor: pointer;
}

.run-preflight__card--active {
  border-color: var(--preflight-primary-color);
  box-shadow: inset 0 0 0 1px var(--preflight-primary-color);
}

.run-preflight__card-title {
  font-weight: 500;
}

.run-preflight__card--active .run-preflight__card-title {
  color: var(--preflight-primary-color);
}

.run-preflight__card-desc {
  color: var(--preflight-secondary-text-color);
  font-size: 13px;
  line-height: 1.5;
}

.run-preflight__guard {
  margin-bottom: 6px;
}

.run-preflight__muted {
  margin: 0;
  color: var(--preflight-secondary-text-color);
  font-size: 13px;
}

/* 阻塞清单：路径名 + 一句原因，整卡可点。 */
.run-preflight__blocks {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.run-preflight__block {
  display: flex;
  flex-direction: column;
  gap: 2px;
  border: 1px solid var(--preflight-border-color);
  border-radius: 6px;
  background: transparent;
  color: inherit;
  cursor: pointer;
  padding: 10px 12px;
  font: inherit;
  text-align: left;
}

.run-preflight__block:hover {
  border-color: var(--preflight-secondary-text-color);
}

.run-preflight__block-name {
  font-weight: 500;
}

.run-preflight__block-reason {
  color: var(--preflight-secondary-text-color);
  font-size: 13px;
  line-height: 1.5;
}

.run-preflight__lead {
  margin: 0 0 12px;
  line-height: 1.6;
}

.run-preflight__start-error {
  margin-bottom: 10px;
}

.run-preflight__footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
