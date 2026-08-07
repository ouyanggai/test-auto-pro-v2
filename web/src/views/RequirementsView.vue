<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NEmpty,
  NList,
  NListItem,
  NSpin,
  NTag,
} from 'naive-ui'
import { useRoute, useRouter } from 'vue-router'

import { fetchExecutionPaths } from '../features/execution-paths/api'
import type { ExecutionPath } from '../features/execution-paths/types'
import { fetchPlan } from '../features/plans/persistence'
import {
  defaultRequirementPath,
  fetchPathRequirements,
  PathRequirementApiError,
  requirementStatusType,
  shouldApplyRequirementResponse,
} from '../features/plans/requirements'
import type { PathRequirements } from '../features/plans/requirements'
import type { PersistedPlan } from '../features/plans/types'

const route = useRoute()
const router = useRouter()
const plan = ref<PersistedPlan | null>(null)
const paths = ref<ExecutionPath[]>([])
const activePathID = ref<string | null>(null)
const requirements = ref<PathRequirements | null>(null)
const pageLoading = ref(false)
const requirementLoading = ref(false)
const pageError = ref('')
const requirementError = ref<PathRequirementApiError | null>(null)
let pageController: AbortController | null = null
let requirementController: AbortController | null = null
let pageVersion = 0
let requirementVersion = 0

const planID = computed(() => String(route.params.id || ''))
const activePath = computed(() => paths.value.find((path) => path.id === activePathID.value) ?? null)

async function loadRequirements(path: ExecutionPath) {
  requirementController?.abort()
  const controller = new AbortController()
  requirementController = controller
  const version = ++requirementVersion
  const requestedPathID = path.id
  requirementLoading.value = true
  requirementError.value = null
  requirements.value = null
  try {
    const result = await fetchPathRequirements(planID.value, requestedPathID, controller.signal)
    if (!shouldApplyRequirementResponse({
      requestedPathId: requestedPathID,
      activePathId: activePathID.value,
      requestVersion: version,
      currentVersion: requirementVersion,
      aborted: controller.signal.aborted,
    })) return
    requirements.value = result
  }
  catch (caught) {
    if (controller.signal.aborted || version !== requirementVersion || activePathID.value !== requestedPathID) return
    requirementError.value = caught instanceof PathRequirementApiError
      ? caught
      : new PathRequirementApiError('暂时无法读取路径要求，请重试', 'TARGET_UNAVAILABLE', true)
  }
  finally {
    if (version === requirementVersion) requirementLoading.value = false
  }
}

function selectPath(path: ExecutionPath) {
  if (activePathID.value === path.id && requirements.value) return
  activePathID.value = path.id
  void loadRequirements(path)
}

async function loadPage() {
  pageController?.abort()
  requirementController?.abort()
  const controller = new AbortController()
  pageController = controller
  const version = ++pageVersion
  requirementVersion++
  pageLoading.value = true
  pageError.value = ''
  requirementError.value = null
  plan.value = null
  paths.value = []
  activePathID.value = null
  requirements.value = null
  try {
    const [storedPlan, storedPaths] = await Promise.all([
      fetchPlan(planID.value, controller.signal),
      fetchExecutionPaths(planID.value, controller.signal),
    ])
    if (controller.signal.aborted || version !== pageVersion) return
    plan.value = storedPlan
    paths.value = [...storedPaths].sort((left, right) => left.sequenceNo - right.sequenceNo)
    const firstPath = defaultRequirementPath(paths.value)
    if (firstPath) {
      activePathID.value = firstPath.id
      await loadRequirements(firstPath)
    }
  }
  catch {
    if (controller.signal.aborted || version !== pageVersion) return
    pageError.value = '暂时无法读取计划或执行路径，请重试'
  }
  finally {
    if (version === pageVersion) pageLoading.value = false
  }
}

function retryRequirements() {
  if (activePath.value) void loadRequirements(activePath.value)
}

watch(planID, () => { void loadPage() }, { immediate: true })
onBeforeUnmount(() => {
  pageController?.abort()
  requirementController?.abort()
})
</script>

<template>
  <section class="requirements-page">
    <div class="requirements-back-bar">
      <n-button text type="primary" @click="router.push(`/plans/${planID}/paths`)">返回路径选择</n-button>
    </div>

    <n-spin :show="pageLoading">
      <template v-if="plan">
        <header class="requirements-heading">
          <div>
            <h1>路径要求核对</h1>
            <p>{{ plan.name }} · {{ plan.targetObjectName }}</p>
          </div>
          <n-tag size="small" :bordered="false">只读核对</n-tag>
        </header>

        <n-empty v-if="paths.length === 0" class="requirements-empty" description="尚未保存执行路径，请先返回选择并保存路径">
          <template #extra>
            <n-button type="primary" @click="router.push(`/plans/${planID}/paths`)">返回选择路径</n-button>
          </template>
        </n-empty>

        <div v-else class="requirements-layout">
          <aside class="requirements-paths" aria-label="已保存路径">
            <h2>已保存路径</h2>
            <n-list class="requirements-path-list" hoverable clickable>
              <n-list-item v-for="path in paths" :key="path.id" @click="selectPath(path)">
                <button
                  type="button"
                  class="requirements-path-button"
                  :class="{ 'requirements-path-button--active': activePathID === path.id }"
                  :aria-current="activePathID === path.id ? 'true' : undefined"
                  @click.stop="selectPath(path)"
                >
                  <span>#{{ path.sequenceNo }}</span>
                  <strong :title="path.name || `路径 ${path.sequenceNo}`">{{ path.name || `路径 ${path.sequenceNo}` }}</strong>
                </button>
              </n-list-item>
            </n-list>
          </aside>

          <main class="requirements-detail" aria-live="polite">
            <div v-if="requirementLoading" class="requirements-state">
              <n-spin size="large" description="正在核对当前真实要求" />
            </div>
            <n-alert v-else-if="requirementError" :type="requirementError.code === 'EXECUTION_PATH_INVALID' ? 'warning' : 'error'" :show-icon="false">
              {{ requirementError.message }}
              <div class="requirements-error-actions">
                <n-button v-if="requirementError.retryable" text type="primary" @click="retryRequirements">重试</n-button>
                <n-button v-else text type="primary" @click="router.push(`/plans/${planID}/paths`)">返回路径选择</n-button>
              </div>
            </n-alert>
            <template v-else-if="requirements">
              <header class="requirements-detail__heading">
                <div>
                  <h2>#{{ requirements.path.sequenceNo }} {{ requirements.path.name || `路径 ${requirements.path.sequenceNo}` }}</h2>
                  <p>以下内容来自当前目标流程配置，不会保存为快照。</p>
                </div>
                <div class="requirements-summary" aria-label="要求状态汇总">
                  <n-tag
                    v-for="item in requirements.summary"
                    :key="item.status"
                    size="small"
                    :type="requirementStatusType(item.status)"
                    :bordered="false"
                  >
                    {{ item.status }} {{ item.count }}
                  </n-tag>
                </div>
              </header>

              <section
                v-for="(group, groupIndex) in requirements.groups"
                :key="`requirement-group-${groupIndex}`"
                class="requirement-group"
              >
                <header>
                  <h3>{{ group.title }}</h3>
                  <n-tag size="small" :bordered="false">{{ group.kind === 'main' ? '主线' : '并行' }}</n-tag>
                </header>
                <div class="requirement-nodes">
                  <article
                    v-for="(node, nodeIndex) in group.nodes"
                    :key="`requirement-node-${groupIndex}-${nodeIndex}`"
                    class="requirement-node"
                  >
                    <div class="requirement-node__title">
                      <strong>{{ node.name }}</strong>
                      <span>{{ node.typeName }}</span>
                    </div>
                    <n-empty v-if="node.items.length === 0" size="small" description="无需额外配置" />
                    <ul v-else>
                      <li v-for="(item, index) in node.items" :key="`${item.category}-${item.title}-${index}`">
                        <div>
                          <span class="requirement-category">{{ item.category }}</span>
                          <strong>{{ item.title }}</strong>
                        </div>
                        <p>{{ item.detail }}</p>
                        <n-tag size="small" :type="requirementStatusType(item.status)" :bordered="false">{{ item.status }}</n-tag>
                      </li>
                    </ul>
                  </article>
                </div>
              </section>
            </template>
          </main>
        </div>
      </template>

      <n-empty v-else-if="!pageLoading" class="requirements-empty" :description="pageError || '暂时无法读取计划'">
        <template #extra>
          <n-button type="primary" secondary @click="loadPage">重试</n-button>
        </template>
      </n-empty>
    </n-spin>
  </section>
</template>

<style scoped>
.requirements-page {
  width: 100%;
  min-width: 0;
}

.requirements-back-bar {
  position: sticky;
  top: -32px;
  z-index: 5;
  margin: -32px -48px 24px;
  padding: 16px 48px 12px;
  background: var(--n-color);
  border-bottom: 1px solid var(--n-border-color);
}

.requirements-heading,
.requirements-detail__heading,
.requirement-group > header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
}

.requirements-heading {
  margin-bottom: 24px;
}

.requirements-heading h1,
.requirements-detail__heading h2,
.requirement-group h3,
.requirements-paths h2 {
  margin: 0;
  font-weight: 600;
}

.requirements-heading h1 {
  margin-bottom: 8px;
  font-size: 28px;
}

.requirements-heading p,
.requirements-detail__heading p {
  margin: 0;
  color: var(--n-text-color-2);
}

.requirements-layout {
  display: grid;
  grid-template-columns: 250px minmax(0, 1fr);
  gap: 24px;
  height: min(620px, calc(100dvh - 260px));
  min-height: 0;
  overflow: hidden;
}

.requirements-paths {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  border-right: 1px solid var(--n-border-color);
  padding-right: 20px;
}

.requirements-paths h2 {
  margin-bottom: 12px;
  font-size: 16px;
}

.requirements-path-list {
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
}

.requirements-path-button {
  display: grid;
  grid-template-columns: 40px minmax(0, 1fr);
  gap: 8px;
  width: 100%;
  padding: 4px 0;
  border: 0;
  color: var(--n-text-color);
  background: transparent;
  text-align: left;
  cursor: pointer;
}

.requirements-path-button strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.requirements-path-button--active {
  color: var(--n-primary-color);
}

.requirements-detail {
  min-width: 0;
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
  padding-right: 4px;
}

.requirements-error-actions {
  margin-top: 8px;
}

.requirements-state,
.requirements-empty {
  display: grid;
  min-height: 420px;
  place-items: center;
}

.requirements-detail__heading {
  margin-bottom: 20px;
}

.requirements-detail__heading h2 {
  margin-bottom: 6px;
  font-size: 20px;
}

.requirements-summary {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.requirement-group {
  border-top: 1px solid var(--n-border-color);
  padding: 20px 0 8px;
}

.requirement-group > header {
  margin-bottom: 12px;
}

.requirement-group h3 {
  font-size: 17px;
}

.requirement-nodes {
  display: grid;
  gap: 12px;
}

.requirement-node {
  display: grid;
  grid-template-columns: 180px minmax(0, 1fr);
  gap: 16px;
  border-bottom: 1px solid var(--n-divider-color);
  padding: 12px 0 16px;
}

.requirement-node__title {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.requirement-node__title span,
.requirement-category {
  color: var(--n-text-color-3);
  font-size: 12px;
}

.requirement-node ul {
  display: grid;
  gap: 12px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.requirement-node li {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 6px 16px;
}

.requirement-node li > div {
  display: flex;
  align-items: center;
  gap: 8px;
}

.requirement-node p {
  grid-column: 1;
  margin: 0;
  color: var(--n-text-color-2);
  line-height: 1.65;
}

.requirement-node .n-tag {
  grid-column: 2;
  grid-row: 1 / span 2;
  align-self: start;
}

@media (max-width: 900px) {
  .requirements-layout {
    grid-template-columns: 1fr;
    height: auto;
    overflow: visible;
  }

  .requirements-paths {
    border-right: 0;
    border-bottom: 1px solid var(--n-border-color);
    padding: 0 0 16px;
    overflow: visible;
  }

  .requirements-path-list,
  .requirements-detail {
    overflow: visible;
  }

  .requirement-node {
    grid-template-columns: 1fr;
  }
}

@media (prefers-reduced-motion: reduce) {
  .requirements-page * {
    scroll-behavior: auto;
  }
}
</style>
