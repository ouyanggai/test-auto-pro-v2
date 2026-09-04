<script setup lang="ts">
import { NAlert, NButton, NCard, NCollapse, NCollapseItem, NEmpty, NSpace, NSpin, NTag, useThemeVars } from 'naive-ui'
import { computed, onBeforeUnmount, ref, watch } from 'vue'

import { fetchPlanRunReadiness, RunReadinessApiError } from './api'
import type { PathRunReadiness, PlanRunReadiness, RunReadinessItem } from './types'

const props = defineProps<{ planId: string }>()
const emit = defineEmits<{ locate: [pathId: string, anchor: string] }>()

const themeVars = useThemeVars()
const readiness = ref<PlanRunReadiness | null>(null)
const loading = ref(false)
const error = ref('')
let controller: AbortController | null = null

const paths = computed<PathRunReadiness[]>(() => readiness.value?.paths ?? [])
const blockedPaths = computed(() => paths.value.filter(path => !path.runnable))
const panelStyle = computed(() => ({
  '--readiness-border-color': themeVars.value.borderColor,
  '--readiness-secondary-text-color': themeVars.value.textColor3,
}))

// load 读取运行准备结论；只读接口，不启动任何运行，切换计划时中止旧请求。
async function load() {
  controller?.abort()
  const active = new AbortController()
  controller = active
  loading.value = true
  error.value = ''
  try {
    const result = await fetchPlanRunReadiness(props.planId, active.signal)
    if (active.signal.aborted) return
    readiness.value = result
  }
  catch (caught) {
    if (active.signal.aborted) return
    error.value = caught instanceof RunReadinessApiError ? caught.message : '暂时无法读取运行准备结论，请重试'
  }
  finally {
    if (controller === active) {
      controller = null
      loading.value = false
    }
  }
}

// locate 把阻塞项交回给页面去定位到那条路径的对应面板，面板本身不做路由跳转。
function locate(path: PathRunReadiness, item: RunReadinessItem) {
  emit('locate', path.pathId, item.anchor)
}

// itemText 组装一条阻塞或提醒的显示文案，名称缺失时只显示原因。
function itemText(item: RunReadinessItem) {
  return item.name ? `${item.name}：${item.reason}` : item.reason
}

defineExpose({ reload: load })

watch(() => props.planId, () => void load(), { immediate: true })
onBeforeUnmount(() => controller?.abort())
</script>

<template>
  <n-card class="run-readiness-panel" data-testid="run-readiness-panel" title="运行准备" size="small" :style="panelStyle" :aria-busy="loading">
    <template #header-extra>
      <n-button size="tiny" secondary :loading="loading" @click="load">重新检查</n-button>
    </template>
    <div v-if="loading && !readiness" class="run-readiness-panel__loading" role="status" aria-live="polite">
      <n-spin size="small" />正在逐条检查执行路径
    </div>
    <n-space v-else vertical size="small">
      <n-alert v-if="error" type="error" :show-icon="false">{{ error }}</n-alert>
      <p v-if="readiness" class="run-readiness-panel__summary" data-testid="run-readiness-summary">{{ readiness.summary }}</p>
      <n-empty v-if="readiness && !blockedPaths.length" size="small" description="所有执行路径都没有阻塞项" />
      <n-collapse v-if="blockedPaths.length" :default-expanded-names="[blockedPaths[0].pathId]">
        <n-collapse-item v-for="path in blockedPaths" :key="path.pathId" :name="path.pathId">
          <template #header>
            <span class="run-readiness-panel__path">{{ path.summary }}</span>
          </template>
          <template #header-extra>
            <n-tag size="small" :bordered="false" type="warning">{{ path.blocks.length }} 项阻塞</n-tag>
          </template>
          <!-- 阻塞与提醒必须分区显示：提醒不影响能否启动，混在一起会让用户以为提醒也要先处理。 -->
          <section class="run-readiness-panel__group" data-testid="run-readiness-blocks">
            <h4>阻塞，必须先处理</h4>
            <button
              v-for="(item, index) in path.blocks"
              :key="`${item.kind}-${index}`"
              type="button"
              class="run-readiness-panel__item run-readiness-panel__item--block"
              @click="locate(path, item)"
            >
              {{ itemText(item) }}
            </button>
          </section>
          <section v-if="path.reminders.length" class="run-readiness-panel__group" data-testid="run-readiness-reminders">
            <h4>仅提醒，不影响启动</h4>
            <p v-for="(item, index) in path.reminders" :key="`${item.kind}-${index}`" class="run-readiness-panel__item">
              {{ itemText(item) }}
            </p>
          </section>
        </n-collapse-item>
      </n-collapse>
    </n-space>
  </n-card>
</template>

<style scoped>
.run-readiness-panel__loading {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--readiness-secondary-text-color);
}

.run-readiness-panel__summary {
  margin: 0;
  line-height: 1.6;
}

.run-readiness-panel__path {
  line-height: 1.6;
}

.run-readiness-panel__group {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding-top: 6px;
}

.run-readiness-panel__group + .run-readiness-panel__group {
  margin-top: 10px;
  border-top: 1px solid var(--readiness-border-color);
}

.run-readiness-panel__group h4 {
  margin: 0;
  font-weight: 500;
  color: var(--readiness-secondary-text-color);
}

.run-readiness-panel__item {
  margin: 0;
  line-height: 1.6;
  text-align: left;
}

.run-readiness-panel__item--block {
  border: 1px solid var(--readiness-border-color);
  border-radius: 4px;
  background: transparent;
  color: inherit;
  cursor: pointer;
  padding: 6px 8px;
  font: inherit;
}

.run-readiness-panel__item--block:hover {
  border-color: var(--readiness-secondary-text-color);
}
</style>
