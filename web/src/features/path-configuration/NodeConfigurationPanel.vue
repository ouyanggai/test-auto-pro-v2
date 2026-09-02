<script setup lang="ts">
import { NAlert, NButton, NCard, NEmpty, NInput, NModal, NPopconfirm, NSelect, NSpace, NTag } from 'naive-ui'
import { computed, ref } from 'vue'
import { AddOutline, ArrowDownOutline, ArrowUpOutline, CloseOutline } from '@vicons/ionicons5'
import { copyPathConfigActions, normalizedActionCount, normalizedPersonStrategy, pathConfigActionsInput, pathConfigurationMessage, pathConfigurationStatusName, resolvedPersonStrategySelection, summarizePathConfigPersonItems } from './logic'
import type { PathConfigActionCycle, PathConfigActionCycleInput, PathConfigActionKind, PathConfigConfiguredActionInput, PathConfigDraft, PathConfigNode, PathConfigPerson, PathConfigPersonStrategyInput } from './types'

const props = defineProps<{ node: PathConfigNode | null; draft: PathConfigDraft; saving: boolean; readOnly: boolean; saveDisabled: boolean; saveAllDisabled: boolean; missingCount: number; saveError: string; saveDetails: Array<{ kind: string; name: string; reason: string }>; savedSuccessfully: boolean; formComplete: boolean; actionCycles: PathConfigActionCycle[] }>()
const emit = defineEmits<{ updatePersonStrategy: [person: PathConfigPerson, value: PathConfigPersonStrategyInput]; updateActionConfiguration: [nodeKey: string, value: PathConfigConfiguredActionInput[]]; updateActionCycles: [value: PathConfigActionCycleInput[]]; save: []; saveAll: []; backToPlan: []; openForm: [] }>()
const actionEditorOpen = ref(false)
const actionDraft = ref<PathConfigConfiguredActionInput[]>([])
const parameterDraft = ref<Record<string, string>>({})
const parameterErrors = ref<Record<string, string>>({})
const allowActionEditorClose = ref(false)

// actions 只保留当前节点的 F-008 动作行。
const savedActions = computed(() => props.node ? (props.draft.actionConfigurations[props.node.key] ?? pathConfigActionsInput(props.node)) : [])
const actions = computed(() => actionDraft.value)
const canAddAction = computed(() => Boolean(props.node?.actionConfiguration.catalog.some(item => item.enabled)) && actions.value.length < 10)
const canSaveActionEditor = computed(() => Object.keys(parameterErrors.value).length === 0)
const disabledActionDefinitions = computed(() => (props.node?.actionConfiguration.catalog ?? []).filter(item => !item.enabled && item.disabledReason))
const actionEditorHasChanges = computed(() => {
  if (JSON.stringify(actionDraft.value) !== JSON.stringify(expandActionRows(savedActions.value))) return true
  return actionDraft.value.some(action => parameterDraft.value[action.key] !== JSON.stringify(action.parameters ?? {}, null, 2))
})
// cycleInputs 只回传服务端派生的循环事实。
// emitActions 复制动作行后更新父级草稿，避免 Vue Proxy 进入请求体。
function emitActions(next: PathConfigConfiguredActionInput[]) { if (props.node) emit('updateActionConfiguration', props.node.key, copyPathConfigActions(next)) }
// expandActionRows 将旧数据中的展示次数展开为独立动作记录，编辑器不再聚合同 kind 动作。
function expandActionRows(input: PathConfigConfiguredActionInput[]): PathConfigConfiguredActionInput[] {
  const rows: PathConfigConfiguredActionInput[] = []
  for (const item of copyPathConfigActions(input)) {
    const count = normalizedActionCount(item.count)
    for (let index = 0; index < count; index += 1) {
      rows.push({ ...item, key: count === 1 ? item.key : `${item.key}#${index + 1}`, count: 1 })
    }
  }
  return rows
}

// openActionEditor 打开弹窗时复制并展开父级草稿，取消不会修改节点面板。
function openActionEditor() {
  actionDraft.value = expandActionRows(savedActions.value)
  parameterDraft.value = Object.fromEntries(actionDraft.value.map(action => [action.key, JSON.stringify(action.parameters ?? {}, null, 2)]))
  parameterErrors.value = {}
  actionEditorOpen.value = true
}
// saveActionEditor 只提交已经确认的弹窗草稿，页面保存按钮负责后端持久化。
function saveActionEditor() { if (!canSaveActionEditor.value) return; emitActions(actionDraft.value); allowActionEditorClose.value = true; actionEditorOpen.value = false }
// closeActionEditor 关闭动作编辑器前确认未保存的独立记录，避免取消误丢排序或参数。
function closeActionEditor() {
  if (actionEditorHasChanges.value && !window.confirm('动作配置尚未保存，确定关闭？')) return
  allowActionEditorClose.value = true
  actionEditorOpen.value = false
}

// handleActionEditorVisibility 拦截 ESC 或点击遮罩触发的关闭请求。
function handleActionEditorVisibility(show: boolean) {
  if (show) { actionEditorOpen.value = true; return }
  if (allowActionEditorClose.value) { allowActionEditorClose.value = false; actionEditorOpen.value = false; return }
  closeActionEditor()
}
// personDraft 返回当前人员策略草稿。
function personDraft(person: PathConfigPerson) { return normalizedPersonStrategy(person, props.draft.personStrategies[person.key]) }
// personOptions 生成不透明人员候选。
function personOptions(person: PathConfigPerson) { return person.options.map(option => ({ label: option.label, value: option.value })) }
// strategyOptions 生成当前模板允许的策略候选。
function strategyOptions(person: PathConfigPerson) { return person.strategies.map(option => ({ label: option.label, value: option.value })) }
// updatePersonStrategy 更新当前节点处理人员草稿。
function updatePersonStrategy(person: PathConfigPerson, patch: Partial<PathConfigPersonStrategyInput>) { const next = { ...personDraft(person), ...patch, key: person.key }; next.selected = resolvedPersonStrategySelection(person, next); emit('updatePersonStrategy', person, next) }
// actionDefinition 返回当前节点的可用动作定义。
function actionDefinition(kind: PathConfigActionKind) { return props.node?.actionConfiguration.catalog.find(item => item.kind === kind && item.enabled) }
// actionPerson 返回动作专用人员目录。
function actionPerson(kind: PathConfigActionKind) { return actionDefinition(kind)?.person }
// actionOptions 返回所有可用动作定义，允许同一语义按独立记录重复编排。
function actionOptions(_index: number) { return (props.node?.actionConfiguration.catalog ?? []).filter(item => item.enabled).map(item => ({ label: item.label, value: item.kind })) }
// updateAction 只更新弹窗草稿，避免用户未确认时污染节点面板。
function updateAction(index: number, patch: Partial<PathConfigConfiguredActionInput>) {
  const next = copyPathConfigActions(actions.value)
  const current = next[index]
  if (!current) return
  const kind = (patch.kind ?? current.kind) as PathConfigActionKind
  const definition = actionDefinition(kind)
  if (!definition) return
  current.kind = kind
  current.count = 1
  current.person = definition.requiresPerson ? (patch.person ?? current.person ?? (definition.person ? personDraft(definition.person) : undefined)) : undefined
  current.actorPolicy = definition.requiresPerson ? (current.person?.strategy || current.actorPolicy) : undefined
  actionDraft.value = next
}
// updateActionPerson 只更新弹窗草稿中的独立人员策略。
function updateActionPerson(index: number, person: PathConfigPerson, patch: Partial<PathConfigPersonStrategyInput>) { const next = copyPathConfigActions(actions.value); if (!next[index]) return; const updated = { ...(next[index].person ?? personDraft(person)), ...patch, key: person.key }; updated.selected = resolvedPersonStrategySelection(person, updated); next[index].person = updated; next[index].actorPolicy = updated.strategy; actionDraft.value = next }
// newActionKey 为每条动作记录生成独立键，键只用于本系统配置幂等，不承载目标临时身份。
function newActionKey() { return globalThis.crypto?.randomUUID?.() ?? `action-local-${Date.now()}-${Math.random().toString(36).slice(2, 8)}` }

// addAction 添加一条独立动作记录，允许与已有记录使用相同 kind。
function addAction() { const definition = props.node?.actionConfiguration.catalog.find(item => item.enabled); if (!definition || actions.value.length >= 10) return; actionDraft.value = [...actions.value, { key: newActionKey(), kind: definition.kind as PathConfigActionKind, count: 1, person: definition.person ? personDraft(definition.person) : undefined }] }
// moveAction 调整动作对应的真实再次到达顺序。
function moveAction(index: number, offset: number) { const target = index + offset; if (target < 0 || target >= actions.value.length) return; const next = copyPathConfigActions(actions.value); [next[index], next[target]] = [next[target], next[index]]; actionDraft.value = next }
// removeAction 删除一个动作行。
function removeAction(index: number) {
  const removed = actions.value[index]
  const next = copyPathConfigActions(actions.value)
  next.splice(index, 1)
  actionDraft.value = next
  if (removed) {
    const { [removed.key]: _draft, ...draftRest } = parameterDraft.value
    const { [removed.key]: _error, ...errorRest } = parameterErrors.value
    parameterDraft.value = draftRest
    parameterErrors.value = errorRest
  }
}
// parametersText 返回当前动作的 JSON 参数草稿，参数仅对应目标接口语义字段。
function parametersText(action: PathConfigConfiguredActionInput) { return parameterDraft.value[action.key] ?? JSON.stringify(action.parameters ?? {}, null, 2) }

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
// itemCount 汇总已解析人员规则，保持侧栏简短。
function itemCount(person: PathConfigPerson) { return summarizePathConfigPersonItems(person.items).total }
</script>

<template>
  <section v-if="node" class="node-configuration-panel">
    <header class="node-configuration-panel__header">
      <h2>{{ node.name }}</h2>
      <n-tag size="small">{{ pathConfigurationStatusName(node.status) }}</n-tag>
    </header>

    <div class="node-configuration-panel__body">
      <n-alert v-if="node.lineBlocked" type="warning" :show-icon="false">前序动作已结束当前线路，本节点无需继续配置。</n-alert>
      <section v-if="node.persons.length" class="node-configuration-panel__section">
        <h3>处理人员</h3>
        <div v-for="person in node.persons" :key="person.key" class="person-row">
          <strong>{{ person.title }}</strong>
          <template v-if="person.editable">
            <div class="person-controls">
								<n-select :value="personDraft(person).strategy" :options="strategyOptions(person)" :disabled="readOnly" @update:value="value => updatePersonStrategy(person, { strategy: value })" />
								<n-select v-if="personDraft(person).strategy === 'manual'" :multiple="person.multiple" :value="person.multiple ? personDraft(person).selected : (personDraft(person).selected[0] ?? null)" :options="personOptions(person)" :disabled="readOnly" @update:value="value => updatePersonStrategy(person, { selected: Array.isArray(value) ? value : (value ? [value] : []) })" />
            </div>
          </template>
          <p v-else>{{ person.detail }}</p>
          <small v-if="itemCount(person)">已解析 {{ itemCount(person) }} 项</small>
          <small v-if="person.note">{{ pathConfigurationMessage(person.note) }}</small>
        </div>
      </section>

      <section class="node-configuration-panel__section">
        <div class="action-section__header">
          <h3>已配置的动作</h3>
					<n-button type="primary" size="small" :disabled="readOnly || node.lineBlocked || !node.actionConfiguration.catalog.length" @click="openActionEditor">动作配置</n-button>
        </div>
        <div v-if="disabledActionDefinitions.length" class="action-disabled-reasons">
          <n-alert v-for="item in disabledActionDefinitions" :key="item.kind" type="warning" :show-icon="false">
            {{ item.label }}：{{ item.disabledReason }}
          </n-alert>
        </div>
        <div v-if="savedActions.length" class="action-summary">
          <n-tag v-for="action in savedActions" :key="action.key" size="small">
            {{ actionDefinition(action.kind)?.label || action.kind }}
          </n-tag>
        </div>
        <span v-else class="muted-text">未添加额外动作</span>
      </section>
    </div>

    <footer class="node-configuration-panel__footer">
      <div class="save-status">
				<n-alert v-if="readOnly" type="info" :show-icon="false">当前计划只能查看</n-alert>
        <n-alert v-if="saveError" type="error" :show-icon="false">{{ pathConfigurationMessage(saveError) }}</n-alert>
        <span v-else-if="missingCount">还有 {{ missingCount }} 项未满足配置要求</span>
      </div>
			<div v-if="!readOnly" class="save-actions">
        <n-button secondary :loading="saving" :disabled="saveAllDisabled" @click="emit('saveAll')">保存全部节点</n-button>
        <n-button class="save-button" type="primary" :loading="saving" :disabled="saveDisabled" @click="emit('save')">保存当前节点</n-button>
      </div>
    </footer>

    <n-modal :show="actionEditorOpen" @update:show="handleActionEditorVisibility">
      <n-card title="动作配置" style="width: min(680px, 94vw)">
        <div v-for="(action, index) in actions" :key="action.key" class="action-row">
          <strong class="action-arrival">第 {{ index + 1 }} 次</strong>
          <n-select class="action-select" :value="action.kind" :options="actionOptions(index)" :disabled="readOnly" @update:value="value => updateAction(index, { kind: value as PathConfigActionKind })" />
          <div class="action-row__actions">
            <n-button quaternary circle title="上移动作" aria-label="上移动作" :disabled="readOnly || index === 0" @click="moveAction(index, -1)"><ArrowUpOutline /></n-button>
            <n-button quaternary circle title="下移动作" aria-label="下移动作" :disabled="readOnly || index === actions.length - 1" @click="moveAction(index, 1)"><ArrowDownOutline /></n-button>
            <n-popconfirm :disabled="readOnly" @positive-click="removeAction(index)">
              <template #trigger><n-button type="error" secondary size="small" title="删除动作" aria-label="删除动作" :disabled="readOnly"><CloseOutline /> 删除</n-button></template>
              删除这个动作配置？
            </n-popconfirm>
          </div>
          <div class="action-parameters">
            <n-input
              type="textarea"
              :value="parametersText(action)"
              :autosize="{ minRows: 2, maxRows: 4 }"
              :disabled="readOnly"
              placeholder="动作参数 JSON"
              @update:value="value => updateActionParameters(index, value)"
            />
            <small v-if="parameterErrors[action.key]" class="action-parameter-error">{{ parameterErrors[action.key] }}</small>
          </div>
          <div v-if="actionPerson(action.kind)" class="action-person-fields">
            <n-select :value="action.person?.strategy || actionPerson(action.kind)!.strategy" :options="strategyOptions(actionPerson(action.kind)!)" :disabled="readOnly" @update:value="value => updateActionPerson(index, actionPerson(action.kind)!, { strategy: value as PathConfigPersonStrategyInput['strategy'] })" />
            <n-select v-if="(action.person?.strategy || actionPerson(action.kind)!.strategy) === 'manual'" :multiple="actionPerson(action.kind)!.multiple" :value="actionPerson(action.kind)!.multiple ? (action.person?.selected || []) : (action.person?.selected?.[0] || null)" :options="personOptions(actionPerson(action.kind)!)" :disabled="readOnly" @update:value="value => updateActionPerson(index, actionPerson(action.kind)!, { selected: Array.isArray(value) ? value : (value ? [value] : []) })" />
          </div>
        </div>
        <template #footer>
          <n-space justify="end"><n-button @click="closeActionEditor">取消</n-button><n-button :disabled="readOnly || !canAddAction" @click="addAction"><AddOutline /> 添加动作</n-button><n-button type="primary" :disabled="readOnly || !canSaveActionEditor" @click="saveActionEditor">保存动作配置</n-button></n-space>
        </template>
      </n-card>
    </n-modal>

    <n-empty v-if="!node.persons.length && !node.actionConfiguration.catalog.length" description="此节点没有需要配置的内容" />
  </section>
</template>

<style scoped>
.node-configuration-panel{height:100%;display:flex;flex-direction:column;gap:12px;padding:16px}.node-configuration-panel__header,.cycle-list li{display:flex;align-items:center;justify-content:space-between;gap:10px}.node-configuration-panel__body{flex:1;min-height:0;overflow:auto;display:flex;flex-direction:column;gap:16px}.node-configuration-panel__section{border-top:1px solid #e5e7eb;padding-top:14px}.action-section__header{display:flex;align-items:center;justify-content:space-between;gap:12px}.person-row{display:flex;flex-direction:column;align-items:stretch;gap:7px;margin-top:12px}.person-controls,.action-person-fields{display:flex;flex-direction:column;gap:7px}.action-summary{display:flex;flex-wrap:wrap;gap:6px;margin:10px 0 0}.action-disabled-reasons{display:flex;flex-direction:column;gap:6px;margin-top:10px}.action-row{display:grid;grid-template-columns:88px minmax(220px,1fr) 148px;gap:10px;align-items:center;padding:10px 0;border-bottom:1px solid #edf0f3}.action-arrival{white-space:nowrap;color:#475569}.action-row__actions{display:flex;justify-content:flex-end;gap:2px}.action-parameters,.action-person-fields{grid-column:2 / -1;max-width:520px}.action-parameter-error{display:block;color:#b42318;margin-top:4px}.muted-text,.node-configuration-panel p,.node-configuration-panel small{color:#64748b}.node-configuration-panel__footer{display:flex;flex-direction:column;align-items:stretch;gap:10px;margin:0 -16px -16px;padding:14px 16px 16px;border-top:1px solid #e5e7eb;background:#fafafa}.save-status{min-height:20px;color:#64748b}.save-button{align-self:flex-end;min-width:132px}.cycle-list{padding-left:0;list-style:none}.node-configuration-panel h2,.node-configuration-panel h3{margin:0}.node-configuration-panel p{margin:0}@media (max-width:680px){.node-configuration-panel{padding:12px}.action-row{grid-template-columns:1fr}.action-arrival{grid-column:1 / -1}.action-select{grid-column:1 / -1}.action-parameters,.action-person-fields{grid-column:1 / -1}.action-row__actions{grid-column:1 / -1;justify-content:flex-start}.save-button{width:100%}.node-configuration-panel__footer{margin:0 -12px -12px;padding:12px}}
/* 操作列固定留出删除文字，避免删除入口只剩不可见的窄图标。 */
.action-row{grid-template-columns:88px minmax(220px,1fr) 148px}
.action-row__actions{align-items:center;gap:4px}
@media (max-width:680px){.action-row__actions{grid-column:1 / -1;justify-content:flex-start}}
.save-actions{display:flex;justify-content:flex-end;gap:8px}
@media (max-width:680px){.save-actions{flex-direction:column-reverse}.save-actions :deep(.n-button){width:100%}.save-button{width:100%}}
</style>
