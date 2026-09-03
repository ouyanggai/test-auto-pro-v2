<script setup lang="ts">
import { AddOutline, ArrowDownOutline, ArrowUpOutline, CloseOutline, ReorderTwoOutline } from '@vicons/ionicons5'
import { NAlert, NButton, NCard, NInputNumber, NModal, NPopconfirm, NSelect, NSpace, NTag } from 'naive-ui'
import { computed, ref } from 'vue'

import { copyPathConfigActions, normalizedPersonStrategy } from './logic'
import type { PathActionContainer, PathConfigActionCatalogItem, PathConfigActionKind, PathConfigConfiguredActionInput, PathConfigPerson, PathConfigPersonStrategyInput } from './types'

const props = defineProps<{
  container: PathActionContainer
  title: string
  savedActions: PathConfigConfiguredActionInput[]
  readOnly: boolean
  blocked: boolean
  personStrategies: Record<string, PathConfigPersonStrategyInput>
  instanceContainer?: PathActionContainer | null
  instanceSavedActions?: PathConfigConfiguredActionInput[]
}>()
const emit = defineEmits<{ update: [containerKey: string, value: PathConfigConfiguredActionInput[]] }>()

const MAX_ACTIONS = 10
const MAX_REPEAT = 9
const editorOpen = ref(false)
const actionDraft = ref<PathConfigConfiguredActionInput[]>([])
const repeatDraft = ref<Record<string, number>>({})
const allowClose = ref(false)
const dragIndex = ref(-1)
const dropIndex = ref(-1)

const nodeCatalog = computed(() => props.container.actionConfiguration.catalog ?? [])
const instanceCatalog = computed(() => props.instanceContainer?.actionConfiguration.catalog ?? [])
const catalog = computed(() => [...nodeCatalog.value, ...instanceCatalog.value])
// 可编排动作排除两类：系统自动节点语义，以及由场景编译器自动插入的恢复动作（如重新提交）。
const selectableCatalog = computed(() => catalog.value.filter(item => !item.systemOnly && !item.systemInserted))
const enabledCatalog = computed(() => selectableCatalog.value.filter(item => item.enabled))
const systemCatalog = computed(() => catalog.value.filter(item => item.systemOnly))
const insertedCatalog = computed(() => catalog.value.filter(item => item.systemInserted))
const disabledCatalog = computed(() => selectableCatalog.value.filter(item => !item.enabled && item.disabledReason))
const actions = computed(() => actionDraft.value)
const savedSummary = computed(() => collapseActions([...props.savedActions, ...(props.instanceSavedActions ?? [])]))
const canAddAction = computed(() => enabledCatalog.value.length > 0 && actions.value.length < MAX_ACTIONS)
const editorDisabled = computed(() => props.readOnly || props.blocked || !selectableCatalog.value.length)
const hasChanges = computed(() => JSON.stringify(expandActions()) !== JSON.stringify([...props.savedActions, ...(props.instanceSavedActions ?? [])]))

// instanceKinds 记录哪些动作属于实例级容器，保存时据此拆分到对应容器。
const instanceKinds = computed(() => new Set(instanceCatalog.value.map(item => item.kind)))

function catalogItem(kind: PathConfigActionKind): PathConfigActionCatalogItem | undefined { return catalog.value.find(item => item.kind === kind) }

function enabledCatalogItem(kind: PathConfigActionKind): PathConfigActionCatalogItem | undefined { return enabledCatalog.value.find(item => item.kind === kind) }

function actionPerson(kind: PathConfigActionKind) { return enabledCatalogItem(kind)?.person }

// actionOptions 分组展示：上半部分当前节点动作，下半部分实例级动作，每项附一句会发生什么。
function actionOptions() {
  const groups = [
    { label: '当前节点动作', items: nodeCatalog.value.filter(item => !item.systemOnly && !item.systemInserted) },
    { label: '实例级动作（作用于整个流程实例）', items: instanceCatalog.value.filter(item => !item.systemOnly && !item.systemInserted) },
  ]
  return groups.filter(group => group.items.length).map(group => ({
    type: 'group' as const,
    label: group.label,
    key: group.label,
    children: group.items.map(item => ({ label: item.label, value: item.kind, disabled: !item.enabled })),
  }))
}

// actionHint 在卡片里展示当前选中动作会发生什么，说明不进下拉选项。
function actionHint(kind: PathConfigActionKind): string {
  const item = catalogItem(kind)
  if (!item) return ''
  return item.enabled ? (item.expectedEffect || item.description) : item.disabledReason
}

function personDraft(person: PathConfigPerson) { return normalizedPersonStrategy(person, props.personStrategies[person.key]) }

function personOptions(person: PathConfigPerson) { return person.options.map(option => ({ label: option.label, value: option.value })) }

function strategyOptions(person: PathConfigPerson) { return person.strategies.map(option => ({ label: option.label, value: option.value })) }

function newActionKey() { return globalThis.crypto?.randomUUID?.() ?? `action-local-${Date.now()}-${Math.random().toString(36).slice(2, 8)}` }

// collapseActions 把连续同类且配置一致的动作折叠为一行并记录次数，界面只显示"动作 ×N"。
function collapseActions(input: PathConfigConfiguredActionInput[]): Array<{ action: PathConfigConfiguredActionInput, count: number }> {
  const result: Array<{ action: PathConfigConfiguredActionInput, count: number }> = []
  for (const action of copyPathConfigActions(input)) {
    const previous = result[result.length - 1]
    if (previous && sameActionShape(previous.action, action)) { previous.count += 1; continue }
    result.push({ action, count: 1 })
  }
  return result
}

// sameActionShape 只有动作语义、人员和参数完全一致时才折叠，避免把不同配置混成次数。
function sameActionShape(left: PathConfigConfiguredActionInput, right: PathConfigConfiguredActionInput) {
  return left.kind === right.kind
    && JSON.stringify(left.person ?? null) === JSON.stringify(right.person ?? null)
    && JSON.stringify(left.parameters ?? null) === JSON.stringify(right.parameters ?? null)
}

// expandActions 把"动作 ×N"展开成 N 条独立记录：执行器逐条执行、逐条重读事实，不在运行时展开次数。
function expandActions(): PathConfigConfiguredActionInput[] {
  const result: PathConfigConfiguredActionInput[] = []
  for (const action of actionDraft.value) {
    const count = Math.max(1, Math.min(MAX_REPEAT, repeatDraft.value[action.key] ?? 1))
    for (let index = 0; index < count; index += 1) {
      result.push({ ...copyPathConfigActions([action])[0], key: index === 0 ? action.key : newActionKey() })
    }
  }
  return result
}

function emitActions() {
  const expanded = expandActions()
  emit('update', props.container.key, expanded.filter(action => !instanceKinds.value.has(action.kind)))
  if (props.instanceContainer) emit('update', props.instanceContainer.key, expanded.filter(action => instanceKinds.value.has(action.kind)))
}

// openEditor 从已保存动作还原草稿，并把连续同类动作折叠回次数。
function openEditor() {
  const collapsed = collapseActions([...props.savedActions, ...(props.instanceSavedActions ?? [])])
  actionDraft.value = collapsed.map(item => item.action)
  repeatDraft.value = Object.fromEntries(collapsed.map(item => [item.action.key, item.count]))
  allowClose.value = false
  editorOpen.value = true
}

function saveEditor() { emitActions(); allowClose.value = true; editorOpen.value = false }

function closeEditor() { allowClose.value = true; editorOpen.value = false }

function handleVisibility(show: boolean) {
  if (show || allowClose.value || !hasChanges.value) { editorOpen.value = show; return }
  editorOpen.value = true
}

function addAction() {
  const first = enabledCatalog.value[0]
  if (!first || actionDraft.value.length >= MAX_ACTIONS) return
  const key = newActionKey()
  actionDraft.value = [...actionDraft.value, { key, kind: first.kind }]
  repeatDraft.value = { ...repeatDraft.value, [key]: 1 }
}

function updateAction(index: number, patch: Partial<PathConfigConfiguredActionInput>) {
  const next = [...actionDraft.value]
  const current = next[index]
  if (!current) return
  next[index] = { ...current, ...patch }
  if (patch.kind && patch.kind !== current.kind) { delete next[index].person; delete next[index].parameters }
  actionDraft.value = next
}

function updateRepeat(action: PathConfigConfiguredActionInput, value: number | null) {
  repeatDraft.value = { ...repeatDraft.value, [action.key]: Math.max(1, Math.min(MAX_REPEAT, Number(value) || 1)) }
}

function updateActionPerson(index: number, person: PathConfigPerson, patch: Partial<PathConfigPersonStrategyInput>) {
  const current = actionDraft.value[index]
  if (!current) return
  const base = current.person ?? personDraft(person)
  updateAction(index, { person: normalizedPersonStrategy(person, { ...base, ...patch, key: person.key }) })
}

function moveAction(index: number, offset: number) { reorderAction(index, index + offset) }

function reorderAction(from: number, to: number) {
  if (from === to || from < 0 || to < 0 || from >= actionDraft.value.length || to >= actionDraft.value.length) return
  const next = [...actionDraft.value]
  const [moved] = next.splice(from, 1)
  next.splice(to, 0, moved)
  actionDraft.value = next
}

function handleDragStart(index: number, event: DragEvent) {
  if (props.readOnly) return
  dragIndex.value = index
  event.dataTransfer?.setData('text/plain', String(index))
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move'
}

function handleDragOver(index: number) { if (dragIndex.value >= 0) dropIndex.value = index }

function handleDrop(index: number) {
  if (dragIndex.value >= 0) reorderAction(dragIndex.value, index)
  handleDragEnd()
}

function handleDragEnd() { dragIndex.value = -1; dropIndex.value = -1 }

function removeAction(index: number) {
  const removed = actionDraft.value[index]
  actionDraft.value = actionDraft.value.filter((_, current) => current !== index)
  if (removed) { const next = { ...repeatDraft.value }; delete next[removed.key]; repeatDraft.value = next }
}
</script>

<template>
  <section class="action-orchestration">
    <div class="action-orchestration__header">
      <h3>{{ title }}</h3>
      <n-button type="primary" size="small" :disabled="editorDisabled" @click="openEditor">动作配置</n-button>
    </div>
    <p v-if="container.actionConfiguration.note" class="action-orchestration__note">{{ container.actionConfiguration.note }}</p>
    <p v-if="instanceContainer" class="action-orchestration__note">下拉上半是当前节点动作，下半是作用于整个实例的动作。</p>

    <div v-if="savedSummary.length" class="action-orchestration__summary">
      <n-tag v-for="(item, index) in savedSummary" :key="item.action.key" size="small">
        {{ index + 1 }}. {{ catalogItem(item.action.kind)?.label || item.action.kind }}{{ item.count > 1 ? ` ×${item.count}` : '' }}
      </n-tag>
    </div>
    <span v-else class="action-orchestration__muted">未添加动作</span>

    <details v-if="selectableCatalog.length" class="action-orchestration__catalog">
      <summary>动作目录（{{ enabledCatalog.length }} / {{ selectableCatalog.length }} 可配置，含不可配置原因）</summary>
      <ul>
        <li v-for="item in insertedCatalog" :key="item.kind">
          <div class="action-orchestration__catalog-head">
            <strong>{{ item.label }}</strong>
            <n-tag size="tiny">系统自动插入</n-tag>
          </div>
          <p>{{ item.systemInsertedReason || '由场景编译器在需要时自动插入' }}</p>
        </li>
        <li v-for="item in systemCatalog" :key="item.kind">
          <div class="action-orchestration__catalog-head">
            <strong>{{ item.label }}</strong>
            <n-tag size="tiny">引擎自动执行</n-tag>
          </div>
          <p>{{ item.description }}</p>
        </li>
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


    <n-modal :show="editorOpen" @update:show="handleVisibility">
      <n-card :title="`${title}（拖动整张卡片调整顺序）`" style="width: min(720px, 94vw)">
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
          <div class="action-row__head">
            <strong class="action-arrival" :title="readOnly ? '' : '按住拖动可调整顺序'">
              <ReorderTwoOutline v-if="!readOnly" class="action-drag-handle" aria-hidden="true" />第 {{ index + 1 }} 步
            </strong>
            <n-select
              class="action-select"
              :value="action.kind"
              :options="actionOptions()"
              :disabled="readOnly"
              @update:value="value => updateAction(index, { kind: value as PathConfigActionKind })"
            />
            <label class="action-repeat">
              执行
              <n-input-number
                size="small"
                :value="repeatDraft[action.key] ?? 1"
                :min="1"
                :max="MAX_REPEAT"
                :show-button="false"
                :disabled="readOnly"
                @update:value="value => updateRepeat(action, value)"
              />
              次
            </label>
            <div class="action-row__actions">
              <n-button quaternary circle title="上移动作" aria-label="上移动作" :disabled="readOnly || index === 0" @click="moveAction(index, -1)"><ArrowUpOutline /></n-button>
              <n-button quaternary circle title="下移动作" aria-label="下移动作" :disabled="readOnly || index === actions.length - 1" @click="moveAction(index, 1)"><ArrowDownOutline /></n-button>
              <n-popconfirm :disabled="readOnly" @positive-click="removeAction(index)">
                <template #trigger><n-button type="error" quaternary circle title="删除动作" aria-label="删除动作" :disabled="readOnly"><CloseOutline /></n-button></template>
                删除这个动作配置？
              </n-popconfirm>
            </div>
          </div>
          <small class="action-row__hint">{{ actionHint(action.kind) }}</small>
          <div v-if="actionPerson(action.kind)" class="action-person-fields">
            <n-select :value="action.person?.strategy || actionPerson(action.kind)!.strategy" :options="strategyOptions(actionPerson(action.kind)!)" :disabled="readOnly" @update:value="value => updateActionPerson(index, actionPerson(action.kind)!, { strategy: value as PathConfigPersonStrategyInput['strategy'] })" />
            <n-select v-if="(action.person?.strategy || actionPerson(action.kind)!.strategy) === 'manual'" :multiple="actionPerson(action.kind)!.multiple" :value="actionPerson(action.kind)!.multiple ? (action.person?.selected || []) : (action.person?.selected?.[0] || null)" :options="personOptions(actionPerson(action.kind)!)" :disabled="readOnly" @update:value="value => updateActionPerson(index, actionPerson(action.kind)!, { selected: Array.isArray(value) ? value : (value ? [value] : []) })" />
          </div>
        </div>
        <n-alert v-if="!actions.length" type="info" :show-icon="false">尚未添加动作，添加后可拖动整张卡片或使用上下按钮调整顺序。</n-alert>
        <template #footer>
          <n-space justify="end">
            <n-button @click="closeEditor">取消</n-button>
            <n-button :disabled="readOnly || !canAddAction" @click="addAction"><AddOutline /> 添加动作</n-button>
            <n-button type="primary" :disabled="readOnly" @click="saveEditor">保存动作配置</n-button>
          </n-space>
        </template>
      </n-card>
    </n-modal>
  </section>
</template>

<style scoped>
.action-orchestration{display:flex;flex-direction:column;gap:8px}
.action-orchestration__header{display:flex;align-items:center;justify-content:space-between;gap:12px}
.action-orchestration__header h3{margin:0;font-size:15px}
.action-orchestration__note,.action-orchestration__hint{margin:0;font-size:12px;opacity:.75}
.action-orchestration__muted{font-size:12px;opacity:.6}
.action-orchestration__summary{display:flex;flex-wrap:wrap;gap:6px}
.action-orchestration__catalog{font-size:12px}
.action-orchestration__catalog ul{display:grid;gap:8px;margin:8px 0 0;padding-left:16px}
.action-orchestration__catalog-head{display:flex;align-items:center;gap:6px}
.action-orchestration__catalog p{margin:2px 0}
.action-orchestration__blocked{color:var(--path-config-warning-color,#d97706)}
.action-orchestration__reasons{display:grid;gap:6px}
.action-row{display:grid;gap:6px;padding:10px 12px;margin-bottom:10px;border:1px solid var(--path-config-border-color,#e5e7eb);border-radius:6px;cursor:grab}
.action-row--dragging{opacity:.6}
.action-row--drop{border-color:var(--path-config-primary-color,#2080f0)}
.action-row__head{display:flex;align-items:center;gap:10px}
.action-arrival{display:flex;align-items:center;gap:4px;flex:0 0 auto;font-size:13px}
.action-drag-handle{width:14px;height:14px}
.action-select{flex:1 1 auto;min-width:0}
.action-repeat{display:flex;align-items:center;gap:4px;flex:0 0 auto;font-size:12px;opacity:.85}
.action-repeat :deep(.n-input-number){width:56px}
.action-row__actions{display:flex;flex:0 0 auto;gap:4px}
.action-row__hint{font-size:12px;opacity:.7}
.action-person-fields{display:flex;flex-wrap:wrap;gap:8px}
.action-person-fields :deep(.n-select){min-width:180px}
</style>
