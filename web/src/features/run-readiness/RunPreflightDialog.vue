<script setup lang="ts">
import { NAlert, NButton, NCheckbox, NCollapse, NCollapseItem, NEmpty, NInput, NModal, NRadio, NRadioGroup, NResult, NSpace, NSpin, NTag, useThemeVars } from 'naive-ui'
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

// F-017：模式三选一（默认单步——最保守），启动前断点预置。
// 首次写断点默认开启可关闭；路径偏离断点强制开启不可关闭（不上 preset，由后端强制内置）。
const mode = ref<string>('single_step')
const modeOptions = [
  { label: '单步运行（每一步执行前都停）', value: 'single_step' },
  { label: '自动运行（连续执行，首个写请求之前必停）', value: 'auto' },
  { label: '人工控制（停在第一步之前，随时放行或连续）', value: 'manual_control' },
]
const firstWriteBreakpoint = ref(true)

const themeVars = useThemeVars()
const readiness = ref<PlanRunReadiness | null>(null)
const loading = ref(false)
const error = ref('')
let controller: AbortController | null = null

const blockedPaths = computed<PathRunReadiness[]>(() => (readiness.value?.paths ?? []).filter(path => !path.runnable))
const reminderPaths = computed<PathRunReadiness[]>(() => (readiness.value?.paths ?? []).filter(path => path.reminders.length > 0))
// allClear 要求确实有勾选路径且全部可运行：空计划与全部阻塞一样不能启动，也不能显示成功结论。
const allClear = computed(() => Boolean(readiness.value) && blockedPaths.value.length === 0 && (readiness.value?.totalCount ?? 0) > 0)
// 宽度必须写成行内样式：NModal 的卡片是 teleport 出去渲染的，scoped 样式选不中它，
// 只靠 class 设宽度会退化成撑满整屏。
const dialogStyle = computed(() => ({
  width: '620px',
  maxWidth: 'calc(100vw - 48px)',
  '--preflight-border-color': themeVars.value.borderColor,
  '--preflight-secondary-text-color': themeVars.value.textColor3,
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

// startSelectedRun 启动全部勾选且可执行的路径（F-020 多路径）：
// 串并方式来自计划配置，调度与失败隔离由后端负责；启动前服务端会再次复验运行准备结论。
// 幂等键每次启动生成一次：同一次点击的重试不会创建第二个运行。
async function startSelectedRun() {
  const targets = (readiness.value?.paths ?? []).filter(path => path.runnable)
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

// itemText 名称缺失时只显示原因，绝不显示内部标识。
function itemText(item: RunReadinessItem) {
  return item.name ? `${item.name}：${item.reason}` : item.reason
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
    title="运行前检查"
    class="run-preflight"
    data-testid="run-preflight-dialog"
    :style="dialogStyle"
    :bordered="false"
    size="small"
    :mask-closable="!loading"
    @update:show="value => emit('update:show', value)"
  >
    <n-spin :show="loading" class="run-preflight__body">
      <n-space vertical size="small">
        <n-alert v-if="error" type="error" :show-icon="false">{{ error }}</n-alert>
        <p v-if="readiness" class="run-preflight__summary" data-testid="run-preflight-summary">
          本次勾选 {{ readiness.totalCount }} 条执行路径：{{ readiness.summary }}
        </p>
        <n-result
          v-if="allClear"
          status="success"
          size="small"
          title="勾选的执行路径都可以运行"
          description="下面的提醒不影响运行，看过就可以继续。"
        />
        <n-collapse v-if="blockedPaths.length" :default-expanded-names="[blockedPaths[0].pathId]">
          <n-collapse-item v-for="path in blockedPaths" :key="path.pathId" :name="path.pathId">
            <template #header>
              <span>{{ path.summary }}</span>
            </template>
            <template #header-extra>
              <n-tag size="small" :bordered="false" type="warning">{{ path.blocks.length }} 项需要先处理</n-tag>
            </template>
            <section class="run-preflight__group" data-testid="run-readiness-blocks">
              <button
                v-for="(item, index) in path.blocks"
                :key="`${item.kind}-${index}`"
                type="button"
                class="run-preflight__item run-preflight__item--block"
                @click="locate(path.pathId, item)"
              >
                {{ itemText(item) }}
              </button>
            </section>
          </n-collapse-item>
        </n-collapse>
        <!-- 提醒与阻塞分区：提醒不影响运行，混在一起会让用户以为也要先处理。 -->
        <section v-if="reminderPaths.length" class="run-preflight__group" data-testid="run-readiness-reminders">
          <h4>以下只是提醒，不影响运行</h4>
          <template v-for="path in reminderPaths" :key="path.pathId">
            <p v-for="(item, index) in path.reminders" :key="`${item.kind}-${index}`" class="run-preflight__item">
              {{ path.pathName }} · {{ itemText(item) }}
            </p>
          </template>
        </section>
        <n-empty v-if="readiness && !blockedPaths.length && !reminderPaths.length" size="small" description="没有需要处理的事项" />
      </n-space>
    </n-spin>
    <template #footer>
      <div class="run-preflight__mode-section">
        <h4>运行模式</h4>
        <n-radio-group v-model:value="mode" class="run-preflight__modes" name="run-mode">
          <n-radio v-for="option in modeOptions" :key="option.value" :value="option.value">{{ option.label }}</n-radio>
        </n-radio-group>
        <h4>启动前断点</h4>
        <n-checkbox v-model:checked="firstWriteBreakpoint">
          首次写断点（默认开启）：第一个写请求之前必停——这是安全阀
        </n-checkbox>
        <p>路径偏离断点强制开启：实际命中分支与已配置路径不一致时强制停下，不可关闭。</p>
        <p>节点、步骤与动作断点在启动后的运行画布上就地挂载：点击画布节点即可挂节点断点。</p>
      </div>
      <n-alert v-if="startError" type="error" :show-icon="false">{{ startError }}</n-alert>
      <n-space justify="end">
        <n-button size="small" :loading="loading" @click="runCheck">重新检查</n-button>
        <n-button size="small" @click="emit('update:show', false)">关闭</n-button>
        <!-- F-016/F-020：运行前检查通过后启动全部勾选的可执行路径，多路径按计划配置串行或并行。 -->
        <n-button
          size="small"
          type="primary"
          :loading="starting"
          :disabled="!allClear"
          :title="allClear ? '按所选模式启动全部勾选的可执行路径' : '存在阻塞项，不能启动'"
          @click="startSelectedRun"
        >开始运行</n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<style scoped>
.run-preflight__body {
  max-height: 60vh;
  overflow-y: auto;
}

.run-preflight__summary {
  margin: 0;
  line-height: 1.6;
}

.run-preflight__group {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding-top: 6px;
}

.run-preflight__group h4 {
  margin: 0;
  font-weight: 500;
  color: var(--preflight-secondary-text-color);
}

/* 模式三选一纵向排列：三个说明都很长，横排会挤成一行读不下去。 */
.run-preflight__modes {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.run-preflight__mode-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 12px;
}

.run-preflight__mode-section h4 {
  margin: 0;
  font-weight: 500;
  color: var(--preflight-secondary-text-color);
}

.run-preflight__mode-section p {
  margin: 0;
  line-height: 1.6;
  color: var(--preflight-secondary-text-color);
}

.run-preflight__item {
  margin: 0;
  line-height: 1.6;
  text-align: left;
  color: var(--preflight-secondary-text-color);
}

.run-preflight__item--block {
  border: 1px solid var(--preflight-border-color);
  border-radius: 4px;
  background: transparent;
  color: inherit;
  cursor: pointer;
  padding: 6px 8px;
  font: inherit;
}

.run-preflight__item--block:hover {
  border-color: var(--preflight-secondary-text-color);
}
</style>
