<script setup lang="ts">
import { NAlert, NButton, NCard, NCollapse, NCollapseItem, NEmpty, NModal, NResult, NSpace, NSpin, NTag, useThemeVars } from 'naive-ui'
import { computed, ref, watch } from 'vue'

import { fetchPlanRunReadiness, RunReadinessApiError } from './api'
import type { PathRunReadiness, PlanRunReadiness, RunReadinessItem } from './types'

const props = defineProps<{ show: boolean, planId: string, pathIds: string[] }>()
const emit = defineEmits<{ 'update:show': [value: boolean], locate: [pathId: string, anchor: string] }>()

const themeVars = useThemeVars()
const readiness = ref<PlanRunReadiness | null>(null)
const loading = ref(false)
const error = ref('')
let controller: AbortController | null = null

const blockedPaths = computed<PathRunReadiness[]>(() => (readiness.value?.paths ?? []).filter(path => !path.runnable))
const reminderPaths = computed<PathRunReadiness[]>(() => (readiness.value?.paths ?? []).filter(path => path.reminders.length > 0))
const allClear = computed(() => Boolean(readiness.value) && blockedPaths.value.length === 0)
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
      <n-space justify="end">
        <n-button size="small" :loading="loading" @click="runCheck">重新检查</n-button>
        <n-button size="small" @click="emit('update:show', false)">关闭</n-button>
        <!-- 真正的启动属于 F-016，这里明确说明当前只做检查，不给一个点了没反应的按钮。 -->
        <n-button size="small" type="primary" disabled>开始运行（执行器切片交付后可用）</n-button>
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
