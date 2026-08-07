<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import {
  NAlert,
  NButton,
  NInput,
  NInputNumber,
  NSelect,
  NSwitch,
  NRadioGroup,
  NRadioButton,
  NSpin,
  useMessage,
} from 'naive-ui'
import { useRoute, useRouter } from 'vue-router'

import {
  fetchPathConfiguration,
  PathConfigApiError,
  savePathConfiguration,
} from '../features/path-configuration/api'
import {
  allEditableFieldsFilled,
  buildPathConfigSavePayload,
  encodePathConfigValue,
  hasPathConfigDraftChanges,
  initPathConfigDraft,
  parsePathConfigValue,
} from '../features/path-configuration/logic'
import type {
  PathConfigAction,
  PathConfigDraft,
  PathConfigField,
  PathConfiguration,
} from '../features/path-configuration/types'
import { fetchPlan, PlanApiError } from '../features/plans/persistence'
import type { PersistedPlan } from '../features/plans/types'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const planID = computed(() => String(route.params.planId || ''))
const pathID = computed(() => String(route.params.pathId || ''))
const plan = ref<PersistedPlan | null>(null)
const configuration = ref<PathConfiguration | null>(null)
const draft = ref<PathConfigDraft>({ fields: {}, actions: {} })
const loading = ref(false)
const saving = ref(false)
const pageError = ref('')
const saveError = ref('')
const saveDetails = ref<Array<{ kind: string, name: string, reason: string }>>([])
let loadVersion = 0
let loadController: AbortController | null = null
let saveController: AbortController | null = null
let saveKey = ''

const dirty = computed(() => configuration.value ? hasPathConfigDraftChanges(configuration.value, draft.value) : false)
const requiredState = computed(() => configuration.value
  ? allEditableFieldsFilled(configuration.value, draft.value)
  : { missing: [] as string[], complete: false })
const saveDisabled = computed(() => loading.value
  || saving.value
  || !configuration.value
  || !dirty.value
  || !requiredState.value.complete)

async function loadPage() {
  loadController?.abort()
  const controller = new AbortController()
  loadController = controller
  const version = ++loadVersion
  loading.value = true
  pageError.value = ''
  saveError.value = ''
  saveDetails.value = []
  plan.value = null
  configuration.value = null
  try {
    const [storedPlan, storedConfiguration] = await Promise.all([
      fetchPlan(planID.value, controller.signal),
      fetchPathConfiguration(planID.value, pathID.value, controller.signal),
    ])
    if (controller.signal.aborted || version !== loadVersion) return
    plan.value = storedPlan
    configuration.value = storedConfiguration
    draft.value = initPathConfigDraft(storedConfiguration)
    saveKey = crypto.randomUUID()
    loading.value = false
  }
  catch (caught) {
    if (controller.signal.aborted || version !== loadVersion) return
    loading.value = false
    pageError.value = caught instanceof PlanApiError || caught instanceof PathConfigApiError
      ? caught.message
      : '暂时无法读取路径配置，请重试'
  }
}

async function saveConfiguration() {
  const current = configuration.value
  if (!current || saveDisabled.value) return
  saveController?.abort()
  saveController = new AbortController()
  saving.value = true
  saveError.value = ''
  saveDetails.value = []
  const payload = buildPathConfigSavePayload(current, draft.value)
  try {
    await savePathConfiguration(planID.value, pathID.value, current.revision, payload.fields, payload.actions, saveKey)
    message.success('路径配置已保存')
    // 保存成功后以服务端最新模型刷新，同步修订号与不同意动作后的阻断状态。
    await loadPage()
  }
  catch (caught) {
    if (caught instanceof DOMException && caught.name === 'AbortError') return
    saving.value = false
    if (caught instanceof PathConfigApiError) {
      saveDetails.value = caught.details
      if (caught.code === 'CONFIG_INVALID') {
        // 结构变化或必填不完整时保留草稿，让用户继续补选，不整页重置。
        saveError.value = '流程已变化或配置不完整，需要重新选择后保存'
      }
      else {
        saveError.value = caught.message
      }
    }
    else {
      saveError.value = '保存失败，草稿已保留，请重试'
    }
  }
}

function fieldValue(field: PathConfigField): unknown {
  return parsePathConfigValue(field, draft.value.fields[field.key] ?? '')
}

function updateFieldValue(field: PathConfigField, value: unknown) {
  draft.value.fields[field.key] = encodePathConfigValue(field, value)
}

function actionValue(action: PathConfigAction): string {
  return draft.value.actions[action.key] ?? action.default
}

function updateActionValue(action: PathConfigAction, value: string) {
  draft.value.actions[action.key] = value
}

function backToPaths() {
  router.push('/plans/' + planID.value + '/paths')
}

onBeforeUnmount(() => {
  loadController?.abort()
  saveController?.abort()
})

loadPage()
</script>

<template>
  <main class="path-configuration-page">
    <header class="path-configuration-page__header">
      <div>
        <n-button text type="primary" @click="backToPaths">返回测试计划</n-button>
        <h1>路径数据与节点动作配置</h1>
        <p v-if="plan" class="path-configuration-page__plan">{{ plan.name }}</p>
      </div>
      <div v-if="configuration" class="path-configuration-page__summary">
        <span>序号 #{{ configuration.path.sequenceNo }}</span>
        <span>{{ configuration.path.name }}</span>
        <n-tag :bordered="false" :type="configuration.status === 'affected' ? 'warning' : 'success'" size="small">
          {{ configuration.status === 'affected' ? '需要重新核对' : '已保存配置' }}
        </n-tag>
      </div>
    </header>

    <n-spin :show="loading" class="path-configuration-page__spin">
      <div class="path-configuration-page__body">
        <n-alert v-if="pageError" type="error" :show-icon="false" class="path-configuration-page__alert">
          {{ pageError }}
        </n-alert>
        <n-button v-if="pageError" size="small" class="path-configuration-page__retry" @click="loadPage">重新读取</n-button>

        <template v-if="configuration">
          <n-alert v-for="(warning, index) in configuration.warnings" :key="index" type="warning" :show-icon="false" class="path-configuration-page__alert">
            {{ warning }}
          </n-alert>

          <section v-for="(group, groupIndex) in configuration.groups" :key="'group-' + groupIndex" class="path-configuration-page__group">
            <h2>{{ group.title }}</h2>
            <article v-for="(node, nodeIndex) in group.nodes" :key="'node-' + groupIndex + '-' + nodeIndex" class="path-configuration-node" :class="{ 'path-configuration-node--blocked': node.lineBlocked }">
              <header class="path-configuration-node__header">
                <strong>{{ node.name }}</strong>
                <n-tag size="small" :bordered="false">{{ node.typeName }}</n-tag>
                <n-alert v-if="node.lineBlocked" type="warning" :show-icon="false" size="small">
                  前序节点选择了不同意，本节点及后续不再按原路径继续
                </n-alert>
              </header>

              <div v-if="node.fields.length" class="path-configuration-node__fields">
                <div v-for="field in node.fields" :key="field.key" class="path-configuration-field" :class="{ 'path-configuration-field--affected': field.affected }">
                  <label>
                    <span>{{ field.name }}</span>
                    <n-tag v-if="field.required" size="tiny" :bordered="false" type="error">必填</n-tag>
                    <n-tag v-if="field.affected" size="tiny" :bordered="false" type="warning">受影响</n-tag>
                  </label>
                  <n-input
                    v-if="field.type === 'text'"
                    :value="String(fieldValue(field))"
                    :disabled="node.lineBlocked || !field.editable"
                    :placeholder="field.required ? '请输入必填值' : '选填'"
                    @update:value="(value) => updateFieldValue(field, value)"
                  />
                  <n-input-number
                    v-else-if="field.type === 'number'"
                    :value="typeof fieldValue(field) === 'number' ? fieldValue(field) as number : null"
                    :disabled="node.lineBlocked || !field.editable"
                    :placeholder="field.required ? '请输入必填值' : '选填'"
                    @update:value="(value) => updateFieldValue(field, value)"
                  />
                  <n-input
                    v-else-if="field.type === 'dateTime'"
                    :value="String(fieldValue(field))"
                    :disabled="node.lineBlocked || !field.editable"
                    placeholder="例如 2026-08-07 或 2026-08-07 10:00:00"
                    @update:value="(value) => updateFieldValue(field, value)"
                  />
                  <n-select
                    v-else-if="field.type === 'singleSelect'"
                    :value="String(fieldValue(field))"
                    :options="field.options"
                    :disabled="node.lineBlocked || !field.editable"
                    :placeholder="field.required ? '请选择必填项' : '请选择'"
                    @update:value="(value) => updateFieldValue(field, value)"
                  />
                  <n-select
                    v-else-if="field.type === 'multiSelect'"
                    multiple
                    :value="Array.isArray(fieldValue(field)) ? fieldValue(field) as string[] : []"
                    :options="field.options"
                    :disabled="node.lineBlocked || !field.editable"
                    :placeholder="field.required ? '请选择必填项' : '请选择'"
                    @update:value="(value) => updateFieldValue(field, value)"
                  />
                  <n-switch
                    v-else-if="field.type === 'switch'"
                    :value="fieldValue(field) === true"
                    :disabled="node.lineBlocked || !field.editable"
                    @update:value="(value) => updateFieldValue(field, value)"
                  />
                  <p v-if="field.note" class="path-configuration-field__note">{{ field.note }}</p>
                </div>
              </div>

              <ul v-if="node.gaps.length" class="path-configuration-node__gaps">
                <li v-for="(gap, gapIndex) in node.gaps" :key="'gap-' + gapIndex">
                  <span>{{ gap.name }}</span>
                  <em>{{ gap.reason }}</em>
                </li>
              </ul>

              <div v-if="node.actions.length" class="path-configuration-node__actions">
                <div v-for="action in node.actions" :key="action.key" class="path-configuration-action">
                  <span>{{ action.label }}</span>
                  <n-radio-group
                    v-if="action.kind === 'agree_disagree'"
                    :value="actionValue(action)"
                    :disabled="node.lineBlocked"
                    @update:value="(value) => updateActionValue(action, value as string)"
                  >
                    <n-radio-button
                      v-for="option in action.options"
                      :key="option.value"
                      :value="option.value"
                    >
                      {{ option.label }}
                    </n-radio-button>
                  </n-radio-group>
                  <n-tag v-else :bordered="false" type="info">固定{{ action.current === 'submit' ? '提交' : action.current }}</n-tag>
                  <p v-if="action.kind === 'agree_disagree' && actionValue(action) === 'disagree'" class="path-configuration-action__warning">
                    {{ action.disagreeWarning }}
                  </p>
                </div>
              </div>
            </article>
          </section>
        </template>
      </div>
    </n-spin>

    <footer v-if="configuration" class="path-configuration-page__footer">
      <div class="path-configuration-page__footer-status">
        <n-alert v-if="saveError" type="error" :show-icon="false" size="small">
          {{ saveError }}
          <template v-if="saveDetails.length">
            <ul class="path-configuration-page__details">
              <li v-for="(item, index) in saveDetails" :key="index">{{ item.name }}：{{ item.reason }}</li>
            </ul>
          </template>
        </n-alert>
        <span v-else-if="requiredState.missing.length">还有 {{ requiredState.missing.length }} 个必填字段未填写</span>
        <span v-else-if="dirty">有未保存的修改</span>
        <span v-else>配置已保存</span>
      </div>
      <n-button type="primary" :loading="saving" :disabled="saveDisabled" @click="saveConfiguration">保存配置</n-button>
    </footer>
  </main>
</template>

<style scoped>
.path-configuration-page {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: var(--bg-color, #ffffff);
  color: var(--text-color, #333333);
}

.path-configuration-page__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 24px;
  border-bottom: 1px solid var(--border-color, rgba(0, 0, 0, 0.08));
}

.path-configuration-page__header h1 {
  margin: 8px 0 4px;
  font-size: 20px;
  line-height: 1.3;
}

.path-configuration-page__plan {
  margin: 0;
  color: var(--text-color-2, #666666);
}

.path-configuration-page__summary {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-color-2, #666666);
}

.path-configuration-page__body {
  flex: 1;
  overflow: auto;
  padding: 16px 24px 88px;
}

.path-configuration-page__alert {
  margin-bottom: 12px;
}

.path-configuration-page__retry {
  margin-bottom: 12px;
}

.path-configuration-page__group {
  margin-bottom: 20px;
}

.path-configuration-page__group h2 {
  margin: 0 0 8px;
  font-size: 16px;
}

.path-configuration-node {
  border: 1px solid var(--border-color, rgba(0, 0, 0, 0.08));
  border-radius: 8px;
  padding: 12px 16px;
  margin-bottom: 12px;
  background: var(--card-color, rgba(0, 0, 0, 0.02));
}

.path-configuration-node--blocked {
  opacity: 0.72;
}

.path-configuration-node__header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}

.path-configuration-node__fields {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
}

.path-configuration-field label {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 4px;
  font-size: 13px;
}

.path-configuration-field--affected {
  outline: 1px solid var(--warning-color, #f0a020);
  border-radius: 6px;
  padding: 6px;
}

.path-configuration-field__note,
.path-configuration-action__warning {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--warning-color, #b8740a);
}

.path-configuration-node__gaps {
  list-style: none;
  margin: 8px 0 0;
  padding: 0;
  display: grid;
  gap: 6px;
}

.path-configuration-node__gaps li {
  display: flex;
  gap: 8px;
  font-size: 13px;
  color: var(--text-color-2, #666666);
}

.path-configuration-node__gaps em {
  font-style: normal;
  color: var(--warning-color, #b8740a);
}

.path-configuration-node__actions {
  margin-top: 10px;
}

.path-configuration-action {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.path-configuration-page__footer {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 24px;
  border-top: 1px solid var(--border-color, rgba(0, 0, 0, 0.08));
  background: var(--bg-color, #ffffff);
}

.path-configuration-page__footer-status {
  flex: 1;
  font-size: 13px;
}

.path-configuration-page__details {
  margin: 4px 0 0;
  padding-left: 18px;
}
</style>
