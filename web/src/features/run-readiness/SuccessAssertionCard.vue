<script setup lang="ts">
import { NAlert, NButton, NCard, NInputNumber, NSelect, NSpace, NSpin, NTag, useThemeVars } from 'naive-ui'
import { computed, onBeforeUnmount, ref, watch } from 'vue'

import { fetchSuccessAssertion, RunReadinessApiError, saveSuccessAssertion } from './api'
import type { SuccessAssertionWorkspace } from './types'

const props = defineProps<{ planId: string, pathId: string, disabled?: boolean }>()
const emit = defineEmits<{ saved: [] }>()

const themeVars = useThemeVars()
const workspace = ref<SuccessAssertionWorkspace | null>(null)
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const endNodeKey = ref('')
const expectedStatus = ref('')
const arrivalOrdinal = ref<number | null>(null)
let controller: AbortController | null = null
let saveKey = crypto.randomUUID()

const candidates = computed(() => workspace.value?.endNodeCandidates ?? [])
const statusOptions = computed(() => (workspace.value?.statusOptions ?? []).map(option => ({ label: option.label, value: option.value })))
const candidateOptions = computed(() => candidates.value.map(candidate => ({
  label: candidate.arrivalCount > 1 ? `${candidate.name || candidate.nodeKey}（本路径会到达 ${candidate.arrivalCount} 次）` : candidate.name || candidate.nodeKey,
  value: candidate.nodeKey,
})))
const selectedCandidate = computed(() => candidates.value.find(candidate => candidate.nodeKey === endNodeKey.value) ?? null)
// 只有会到达多次的结束节点才要求指定第几次到达，只到达一次时界面不出现这一项。
const needsOrdinal = computed(() => (selectedCandidate.value?.arrivalCount ?? 1) > 1)
const savedAssertion = computed(() => workspace.value?.assertion ?? null)
const issues = computed(() => workspace.value?.issues ?? [])
const cardStyle = computed(() => ({
  '--assertion-border-color': themeVars.value.borderColor,
  '--assertion-secondary-text-color': themeVars.value.textColor3,
}))

// loadWorkspace 读取真实候选与已保存断言；切换路径时中止旧请求，避免跨路径回写。
async function loadWorkspace() {
  controller?.abort()
  const active = new AbortController()
  controller = active
  loading.value = true
  error.value = ''
  try {
    const result = await fetchSuccessAssertion(props.planId, props.pathId, active.signal)
    if (active.signal.aborted) return
    workspace.value = result
    endNodeKey.value = result.assertion?.endNodeKey ?? ''
    expectedStatus.value = result.assertion?.expectedStatus ?? ''
    arrivalOrdinal.value = result.assertion?.arrivalOrdinal ?? null
  }
  catch (caught) {
    if (active.signal.aborted) return
    error.value = caught instanceof RunReadinessApiError ? caught.message : '暂时无法读取成功断言，请重试'
  }
  finally {
    if (controller === active) {
      controller = null
      loading.value = false
    }
  }
}

// submit 保存断言；校验原因由后端给出，界面不自造提示也不自动修正取值。
async function submit() {
  if (saving.value || props.disabled) return
  saving.value = true
  error.value = ''
  try {
    await saveSuccessAssertion(props.planId, props.pathId, {
      endNodeKey: endNodeKey.value,
      expectedStatus: expectedStatus.value,
      arrivalOrdinal: needsOrdinal.value ? Number(arrivalOrdinal.value ?? 0) : 0,
      revision: savedAssertion.value?.revision ?? 0,
    }, saveKey)
    saveKey = crypto.randomUUID()
    await loadWorkspace()
    emit('saved')
  }
  catch (caught) {
    error.value = caught instanceof RunReadinessApiError ? caught.message : '成功断言保存失败，请重试'
  }
  finally {
    saving.value = false
  }
}

watch(() => [props.planId, props.pathId], () => {
  saveKey = crypto.randomUUID()
  void loadWorkspace()
}, { immediate: true })

onBeforeUnmount(() => controller?.abort())
</script>

<template>
  <n-card class="success-assertion-card" data-testid="success-assertion-card" title="成功断言" size="small" :style="cardStyle" :aria-busy="loading">
    <template #header-extra>
      <n-tag v-if="savedAssertion" size="small" :bordered="false" :type="issues.length ? 'warning' : 'success'">
        {{ issues.length ? '需要重新配置' : '已配置' }}
      </n-tag>
      <n-tag v-else size="small" :bordered="false" type="warning">未配置</n-tag>
    </template>
    <div v-if="loading" class="success-assertion-card__loading" role="status" aria-live="polite">
      <n-spin size="small" />正在读取这条路径的结束节点
    </div>
    <n-space v-else vertical size="small">
      <p class="success-assertion-card__hint">
        这条路径跑成什么样才算成功：跑到哪个结束节点、实例状态是什么。结束节点只能从这条路径真实经过的节点里选。
      </p>
      <n-alert v-if="error" type="error" :show-icon="false">{{ error }}</n-alert>
      <n-alert v-for="issue in issues" :key="issue.reason" type="warning" :show-icon="false">
        {{ issue.name ? `${issue.name}：${issue.reason}` : issue.reason }}
      </n-alert>
      <n-alert v-if="!candidates.length" type="warning" :show-icon="false">
        这条路径在当前真实流程结构里没有结束节点，无法配置成功断言。
      </n-alert>
      <template v-else>
        <label class="success-assertion-card__field">
          <span>结束节点</span>
          <n-select v-model:value="endNodeKey" :options="candidateOptions" :disabled="disabled" placeholder="选择这条路径的结束节点" />
        </label>
        <label class="success-assertion-card__field">
          <span>期望实例状态</span>
          <n-select v-model:value="expectedStatus" :options="statusOptions" :disabled="disabled" placeholder="选择目标平台的实例状态" />
        </label>
        <label v-if="needsOrdinal" class="success-assertion-card__field">
          <span>第几次到达算成功</span>
          <n-input-number
            v-model:value="arrivalOrdinal"
            :min="1"
            :max="selectedCandidate?.arrivalCount ?? 1"
            :disabled="disabled"
            placeholder="这个结束节点在本路径上会到达多次，请指定第几次"
          />
        </label>
        <n-space justify="end">
          <n-button type="primary" size="small" :loading="saving" :disabled="disabled" @click="submit">保存成功断言</n-button>
        </n-space>
      </template>
      <p v-if="savedAssertion" class="success-assertion-card__saved">
        当前配置：跑到「{{ savedAssertion.endNodeName || savedAssertion.endNodeKey }}」，实例状态为「{{ savedAssertion.expectedStatusLabel }}」{{ savedAssertion.arrivalOrdinal > 1 ? `，第 ${savedAssertion.arrivalOrdinal} 次到达算成功` : '' }}
      </p>
    </n-space>
  </n-card>
</template>

<style scoped>
.success-assertion-card__loading {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--assertion-secondary-text-color);
}

.success-assertion-card__hint {
  margin: 0;
  color: var(--assertion-secondary-text-color);
  line-height: 1.6;
}

.success-assertion-card__field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.success-assertion-card__field > span {
  color: var(--assertion-secondary-text-color);
}

.success-assertion-card__saved {
  margin: 0;
  padding-top: 4px;
  border-top: 1px solid var(--assertion-border-color);
  line-height: 1.6;
}
</style>
