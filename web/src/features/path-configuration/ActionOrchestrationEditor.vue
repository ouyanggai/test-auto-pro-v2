<script setup lang="ts">
import { NAlert, NButton, NCard, NInput, NModal, NPopconfirm, NSelect, NSpace, NTag } from 'naive-ui'
import { computed, ref } from 'vue'
import { AddOutline, ArrowDownOutline, ArrowUpOutline, CloseOutline, ReorderTwoOutline } from '@vicons/ionicons5'
import { copyPathConfigActions, normalizedPersonStrategy, resolvedPersonStrategySelection } from './logic'
import type { PathActionContainer, PathConfigActionCatalogItem, PathConfigActionKind, PathConfigConfiguredActionInput, PathConfigPerson, PathConfigPersonStrategyInput } from './types'

const props = defineProps<{ container: PathActionContainer; title: string; savedActions: PathConfigConfiguredActionInput[]; readOnly: boolean; blocked: boolean; personStrategies: Record<string, PathConfigPersonStrategyInput> }>()
const emit = defineEmits<{ update: [containerKey: string, value: PathConfigConfiguredActionInput[]] }>()

const MAX_ACTIONS = 10
const editorOpen = ref(false)
const actionDraft = ref<PathConfigConfiguredActionInput[]>([])
const parameterDraft = ref<Record<string, string>>({})
const parameterErrors = ref<Record<string, string>>({})
const allowClose = ref(false)
const dragIndex = ref(-1)
const dropIndex = ref(-1)

const catalog = computed(() => props.container.actionConfiguration.catalog ?? [])
// selectableCatalog 排除系统自动语义项，只有真实用户动作可以进入编排。
const selectableCatalog = computed(() => catalog.value.filter(item => !item.systemOnly))
const enabledCatalog = computed(() => selectableCatalog.value.filter(item => item.enabled))
const systemCatalog = computed(() => catalog.value.filter(item => item.systemOnly))
const disabledCatalog = computed(() => selectableCatalog.value.filter(item => !item.enabled && item.disabledReason))
const actions = computed(() => actionDraft.value)
const canAddAction = computed(() => enabledCatalog.value.length > 0 && actions.value.length < MAX_ACTIONS)
const canSave = computed(() => Object.keys(parameterErrors.value).length === 0)
const editorDisabled = computed(() => props.readOnly || props.blocked || !selectableCatalog.value.length)
const hasChanges = computed(() => {
  if (JSON.stringify(actionDraft.value) !== JSON.stringify(copyPathConfigActions(props.savedActions))) return true
  return actionDraft.value.some(action => parameterDraft.value[action.key] !== JSON.stringify(action.parameters ?? {}, null, 2))
})

// catalogItem 按动作键定位当前上下文的目录项，禁用项也要能显示真实门禁原因。
function catalogItem(kind: PathConfigActionKind): PathConfigActionCatalogItem | undefined { return catalog.value.find(item => item.kind === kind) }
// enabledCatalogItem 只返回当前允许编排的目录项。
function enabledCatalogItem(kind: PathConfigActionKind): PathConfigActionCatalogItem | undefined { return enabledCatalog.value.find(item => item.kind === kind) }
// actionPerson 返回动作专用人员目录。
function actionPerson(kind: PathConfigActionKind) { return enabledCatalogItem(kind)?.person }
// actionOptions 返回所有可用动作，允许同一语义按多条独立记录重复编排。
function actionOptions() { return enabledCatalog.value.map(item => ({ label: item.label, value: item.kind })) }
// personDraft 返回当前人员策略草稿。
function personDraft(person: PathConfigPerson) { return normalizedPersonStrategy(person, props.personStrategies[person.key]) }
// personOptions 生成不透明人员候选。
function personOptions(person: PathConfigPerson) { return person.options.map(option => ({ label: option.label, value: option.value })) }
// strategyOptions 生成当前模板允许的策略候选。
function strategyOptions(person: PathConfigPerson) { return person.strategies.map(option => ({ label: option.label, value: option.value })) }
// newActionKey 为每条动作记录生成独立键，键只用于本系统配置幂等，不承载目标临时身份。
function newActionKey() { return globalThis.crypto?.randomUUID?.() ?? `action-local-${Date.now()}-${Math.random().toString(36).slice(2, 8)}` }
// emitActions 复制动作行后更新父级草稿，避免 Vue Proxy 进入请求体。
function emitActions(next: PathConfigConfiguredActionInput[]) { emit('update', props.container.key, copyPathConfigActions(next)) }

// openEditor 打开弹窗时复制父级草稿，取消不会修改已确认配置。
function openEditor() {
  actionDraft.value = copyPathConfigActions(props.savedActions)
  parameterDraft.value = Object.fromEntries(actionDraft.value.map(action => [action.key, JSON.stringify(action.parameters ?? {}, null, 2)]))
  parameterErrors.value = {}
  dragIndex.value = -1
  dropIndex.value = -1
  editorOpen.value = true
}
// saveEditor 只提交已经确认的弹窗草稿，页面保存按钮负责后端持久化。
function saveEditor() { if (!canSave.value) return; emitActions(actionDraft.value); allowClose.value = true; editorOpen.value = false }
// closeEditor 关闭前确认未保存的独立记录，避免取消误丢排序或参数。
function closeEditor() {
  if (hasChanges.value && !window.confirm('动作配置尚未保存，确定关闭？')) return
  allowClose.value = true
  editorOpen.value = false
}
// handleVisibility 拦截 ESC 或点击遮罩触发的关闭请求。
function handleVisibility(show: boolean) {
  if (show) { editorOpen.value = true; return }
  if (allowClose.value) { allowClose.value = false; editorOpen.value = false; return }
  closeEditor()
}
// addAction 添加一条独立动作记录，允许与已有记录使用相同动作。
function addAction() {
  const definition = enabledCatalog.value[0]
  if (!definition || actions.value.length >= MAX_ACTIONS) return
  actionDraft.value = [...actions.value, { key: newActionKey(), kind: definition.kind, person: definition.person ? personDraft(definition.person) : undefined }]
}
// updateAction 只更新弹窗草稿，避免用户未确认时污染已确认配置。
function updateAction(index: number, patch: Partial<PathConfigConfiguredActionInput>) {
  const next = copyPathConfigActions(actions.value)
  const current = next[index]
  if (!current) return
  const kind = (patch.kind ?? current.kind) as PathConfigActionKind
  const definition = enabledCatalogItem(kind)
  if (!definition) return
  current.kind = kind
  current.person = definition.requiresPerson ? (patch.person ?? current.person ?? (definition.person ? personDraft(definition.person) : undefined)) : undefined
  current.actorPolicy = definition.requiresPerson ? (current.person?.strategy || current.actorPolicy) : undefined
  actionDraft.value = next
}
// updateActionPerson 只更新弹窗草稿中的独立人员策略。
function updateActionPerson(index: number, person: PathConfigPerson, patch: Partial<PathConfigPersonStrategyInput>) {
  const next = copyPathConfigActions(actions.value)
  if (!next[index]) return
  const updated = { ...(next[index].person ?? personDraft(person)), ...patch, key: person.key }
  updated.selected = resolvedPersonStrategySelection(person, updated)
  next[index].person = updated
  next[index].actorPolicy = updated.strategy
  actionDraft.value = next
}
// moveAction 调整动作在同一主实例上的真实执行顺序。
function moveAction(index: number, offset: number) {
  const target = index + offset
  if (target < 0 || target >= actions.value.length) return
  const next = copyPathConfigActions(actions.value)
  ;[next[index], next[target]] = [next[target], next[index]]
  actionDraft.value = next
}
// reorderAction 把拖拽记录移动到目标位置，保持其余记录的相对顺序。
function reorderAction(from: number, to: number) {
  if (from === to || from < 0 || to < 0 || from >= actions.value.length || to >= actions.value.length) return
  const next = copyPathConfigActions(actions.value)
  const [moved] = next.splice(from, 1)
  next.splice(to, 0, moved)
  actionDraft.value = next
}
// handleDragStart 记录被拖拽的动作行，只读或阻断时不启动拖拽。
function handleDragStart(index: number, event: DragEvent) {
  if (props.readOnly) { event.preventDefault(); return }
  dragIndex.value = index
  dropIndex.value = index
  event.dataTransfer?.setData('text/plain', String(index))
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move'
}
// handleDragOver 高亮当前放置位置。
function handleDragOver(index: number) { if (dragIndex.value >= 0) dropIndex.value = index }
// handleDrop 按拖拽结果重排动作顺序。
function handleDrop(index: number) {
  if (dragIndex.value < 0) return
  reorderAction(dragIndex.value, index)
  dragIndex.value = -1
  dropIndex.value = -1
}
// handleDragEnd 清除拖拽高亮状态。
function handleDragEnd() { dragIndex.value = -1; dropIndex.value = -1 }
// removeAction 删除一个动作行。
function removeAction(index: number) {
  const removed = actions.value[index]
  const next = copyPathConfigActions(actions.value)
  next.splice(index, 1)
  actionDraft.value = next
  if (!removed) return
  const { [removed.key]: _draft, ...draftRest } = parameterDraft.value
  const { [removed.key]: _error, ...errorRest } = parameterErrors.value
  parameterDraft.value = draftRest
  parameterErrors.value = errorRest
}
// parametersText 返回当前动作的 JSON 参数草稿，参数仅对应目标接口语义字段。
function parametersText(action: PathConfigConfiguredActionInput) { return parameterDraft.value[action.key] ?? JSON.stringify(action.parameters ?? {}, null, 2) }
// parameterPlaceholder 用目标接口真实参数名提示当前动作需要填写的内容。
function parameterPlaceholder(kind: PathConfigActionKind) {
  const item = catalogItem(kind)
  if (!item?.parameters.length) return '动作参数 JSON'
  return `动作参数 JSON，目标参数：${item.parameters.join('、')}`
}
// updateActionParameters 解析独立动作参数，非法 JSON 会阻止弹窗保存而不污染已确认草稿。
function updateActionParameters(index: number, value: string) {
  const current = actionDraft.value[index]
  if (!current) return
  parameterDraft.value = { ...parameterDraft.value, [current.key]: value }
  const trimmed = value.trim()
  if (!trimmed) {
    const next = copyPathConfigActions(actions.value)
    delete next[index].parameters
    actionDraft.value = next
    const { [current.key]: _removed, ...rest } = parameterErrors.value
    parameterErrors.value = rest
    return
  }
  try {
    const parsed: unknown = JSON.parse(trimmed)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error('动作参数必须是 JSON 对象')
    const next = copyPathConfigActions(actions.value)
    next[index].parameters = parsed as Record<string, unknown>
    actionDraft.value = next
    const { [current.key]: _removed, ...rest } = parameterErrors.value
    parameterErrors.value = rest
  }
  catch (error) {
    parameterErrors.value = { ...parameterErrors.value, [current.key]: error instanceof Error ? error.message : '动作参数 JSON 无效' }
  }
}
</script>

<template>
  <section class="action-orchestration">
    <div class="action-orchestration__header">
      <h3>{{ title }}</h3>
      <n-button type="primary" size="small" :disabled="editorDisabled" @click="openEditor">动作配置</n-button>
    </div>
    <p v-if="container.actionConfiguration.note" class="action-orchestration__note">{{ container.actionConfiguration.note }}</p>
    <n-alert v-if="container.actionConfiguration.base" type="info" :show-icon="false">
      系统默认动作：{{ container.actionConfiguration.base.label }}（{{ container.actionConfiguration.base.detail }}）
    </n-alert>

    <div v-if="savedActions.length" class="action-orchestration__summary">
      <n-tag v-for="(action, index) in savedActions" :key="action.key" size="small">
        {{ index + 1 }}. {{ catalogItem(action.kind)?.label || action.kind }}
      </n-tag>
    </div>
    <span v-else class="action-orchestration__muted">未添加动作</span>

    <details v-if="selectableCatalog.length" class="action-orchestration__catalog">
      <summary>动作目录与门禁（{{ enabledCatalog.length }} / {{ selectableCatalog.length }} 可配置）</summary>
      <ul>
        <li v-for="item in selectableCatalog" :key="item.kind">
          <div class="action-orchestration__catalog-head">
            <strong>{{ item.label }}</strong>
            <n-tag size="tiny" :type="item.enabled ? 'success' : 'warning'">{{ item.enabled ? '可配置' : '不可配置' }}</n-tag>
            <n-tag v-if="item.requiresReload" size="tiny">需重读事实</n-tag>
          </div>
          <p>{{ item.description }}</p>
          <p v-if="!item.enabled && item.disabledReason" class="action-orchestration__blocked">{{ item.disabledReason }}</p>
          <p v-if="item.runtimeNote" class="action-orchestration__hint">{{ item.runtimeNote }}</p>
          <p v-if="item.expectedEffect">预期结果：{{ item.expectedEffect }}</p>
          <p v-if="item.preconditions.length">
            前置事实：<span v-for="precondition in item.preconditions" :key="precondition.key">{{ precondition.label }}（{{ precondition.present ? '已满足' : '未满足' }}）</span>
          </p>
          <p v-if="item.reloadRequirements.length">重读要求：{{ item.reloadRequirements.join('、') }}</p>
        </li>
      </ul>
    </details>

    <div v-if="disabledCatalog.length" class="action-orchestration__reasons">
      <n-alert v-for="item in disabledCatalog" :key="item.kind" type="warning" :show-icon="false">{{ item.label }}：{{ item.disabledReason }}</n-alert>
    </div>
    <n-alert v-for="item in systemCatalog" :key="item.kind" type="info" :show-icon="false">{{ item.label }}：{{ item.description }}</n-alert>

    <n-modal :show="editorOpen" @update:show="handleVisibility">
      <n-card :title="`${title}（可拖拽调整顺序）`" style="width: min(720px, 94vw)">
        <div
          v-for="(action, index) in actions"
          :key="action.key"
          class="action-row"
          :class="{ 'action-row--dragging': dragIndex === index, 'action-row--drop': dropIndex === index && dragIndex !== index }"
          :draggable="!readOnly"
          @dragstart="event => handleDragStart(index, event)"
          @dragover.prevent="handleDragOver(index)"
          @drop.prevent="handleDrop(index)"
          @dragend="handleDragEnd"
        >
          <strong class="action-arrival" :title="readOnly ? '' : '按住拖动可调整顺序'">
            <ReorderTwoOutline v-if="!readOnly" class="action-drag-handle" aria-hidden="true" />第 {{ index + 1 }} 步
          </strong>
          <n-select class="action-select" :value="action.kind" :options="actionOptions()" :disabled="readOnly" @update:value="value => updateAction(index, { kind: value as PathConfigActionKind })" />
          <div class="action-row__actions">
            <n-button quaternary circle title="上移动作" aria-label="上移动作" :disabled="readOnly || index === 0" @click="moveAction(index, -1)"><ArrowUpOutline /></n-button>
            <n-button quaternary circle title="下移动作" aria-label="下移动作" :disabled="readOnly || index === actions.length - 1" @click="moveAction(index, 1)"><ArrowDownOutline /></n-button>
            <n-popconfirm :disabled="readOnly" @positive-click="removeAction(index)">
              <template #trigger><n-button type="error" secondary size="small" title="删除动作" aria-label="删除动作" :disabled="readOnly"><CloseOutline /> 删除</n-button></template>
              删除这个动作配置？
            </n-popconfirm>
          </div>
          <small v-if="catalogItem(action.kind)?.runtimeNote" class="action-row__hint">{{ catalogItem(action.kind)?.runtimeNote }}</small>
          <div class="action-parameters">
            <n-input
              type="textarea"
              :value="parametersText(action)"
              :autosize="{ minRows: 2, maxRows: 4 }"
              :disabled="readOnly"
              :placeholder="parameterPlaceholder(action.kind)"
              @update:value="value => updateActionParameters(index, value)"
            />
            <small v-if="parameterErrors[action.key]" class="action-parameter-error">{{ parameterErrors[action.key] }}</small>
          </div>
          <div v-if="actionPerson(action.kind)" class="action-person-fields">
            <n-select :value="action.person?.strategy || actionPerson(action.kind)!.strategy" :options="strategyOptions(actionPerson(action.kind)!)" :disabled="readOnly" @update:value="value => updateActionPerson(index, actionPerson(action.kind)!, { strategy: value as PathConfigPersonStrategyInput['strategy'] })" />
            <n-select v-if="(action.person?.strategy || actionPerson(action.kind)!.strategy) === 'manual'" :multiple="actionPerson(action.kind)!.multiple" :value="actionPerson(action.kind)!.multiple ? (action.person?.selected || []) : (action.person?.selected?.[0] || null)" :options="personOptions(actionPerson(action.kind)!)" :disabled="readOnly" @update:value="value => updateActionPerson(index, actionPerson(action.kind)!, { selected: Array.isArray(value) ? value : (value ? [value] : []) })" />
          </div>
        </div>
        <n-alert v-if="!actions.length" type="info" :show-icon="false">尚未添加动作，添加后可拖拽或使用上下按钮调整顺序。</n-alert>
        <template #footer>
          <n-space justify="end">
            <n-button @click="closeEditor">取消</n-button>
            <n-button :disabled="readOnly || !canAddAction" @click="addAction"><AddOutline /> 添加动作</n-button>
            <n-button type="primary" :disabled="readOnly || !canSave" @click="saveEditor">保存动作配置</n-button>
          </n-space>
        </template>
      </n-card>
    </n-modal>
  </section>
</template>

<style scoped>
.action-orchestration{display:flex;flex-direction:column;gap:8px}
.action-orchestration__header{display:flex;align-items:center;justify-content:space-between;gap:12px}
.action-orchestration h3{margin:0}
.action-orchestration p{margin:0}
.action-orchestration__note,.action-orchestration__muted,.action-orchestration__hint{color:#64748b}
.action-orchestration__blocked{color:#b42318}
.action-orchestration__summary{display:flex;flex-wrap:wrap;gap:6px}
.action-orchestration__reasons{display:flex;flex-direction:column;gap:6px}
.action-orchestration__catalog{border:1px solid #e5e7eb;border-radius:6px;padding:8px 10px}
.action-orchestration__catalog summary{cursor:pointer;color:#334155}
.action-orchestration__catalog ul{list-style:none;padding:0;margin:8px 0 0;display:flex;flex-direction:column;gap:10px}
.action-orchestration__catalog li{border-top:1px solid #edf0f3;padding-top:8px;font-size:12px;color:#475569;display:flex;flex-direction:column;gap:3px}
.action-orchestration__catalog-head{display:flex;align-items:center;gap:6px}
.action-row{display:grid;grid-template-columns:112px minmax(220px,1fr) 148px;gap:10px;align-items:center;padding:10px 0;border-bottom:1px solid #edf0f3}
.action-row--dragging{opacity:.5}
.action-row--drop{background:#f1f5f9}
.action-arrival{white-space:nowrap;color:#475569;display:flex;align-items:center;gap:4px;cursor:grab}
.action-drag-handle{width:14px;height:14px}
.action-row__actions{display:flex;justify-content:flex-end;gap:4px;align-items:center}
.action-row__hint{grid-column:1 / -1;color:#64748b}
.action-parameters,.action-person-fields{grid-column:2 / -1;max-width:520px}
.action-person-fields{display:flex;flex-direction:column;gap:7px}
.action-parameter-error{display:block;color:#b42318;margin-top:4px}
@media (max-width:680px){.action-row{grid-template-columns:1fr}.action-arrival,.action-select,.action-parameters,.action-person-fields{grid-column:1 / -1}.action-row__actions{grid-column:1 / -1;justify-content:flex-start}}
</style>
