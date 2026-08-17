<script setup lang="ts">
import { NAlert, NButton, NCard, NEmpty, NInputNumber, NModal, NPopconfirm, NSelect, NSpace, NTag } from 'naive-ui'
import { computed, ref } from 'vue'
import { AddOutline, ArrowDownOutline, ArrowUpOutline, CloseOutline } from '@vicons/ionicons5'
import { copyPathConfigActions, normalizedActionCount, normalizedPersonStrategy, pathConfigActionsInput, pathConfigurationMessage, pathConfigurationStatusName, resolvedPersonStrategySelection, summarizePathConfigPersonItems } from './logic'
import type { PathConfigActionCycle, PathConfigActionCycleInput, PathConfigActionKind, PathConfigConfiguredActionInput, PathConfigDraft, PathConfigNode, PathConfigPerson, PathConfigPersonStrategyInput } from './types'

const props = defineProps<{ node: PathConfigNode | null; draft: PathConfigDraft; saving: boolean; saveDisabled: boolean; missingCount: number; saveError: string; saveDetails: Array<{ kind: string; name: string; reason: string }>; savedSuccessfully: boolean; formComplete: boolean; actionCycles: PathConfigActionCycle[] }>()
const emit = defineEmits<{ updatePersonStrategy: [person: PathConfigPerson, value: PathConfigPersonStrategyInput]; updateActionConfiguration: [nodeKey: string, value: PathConfigConfiguredActionInput[]]; updateActionCycles: [value: PathConfigActionCycleInput[]]; save: []; backToPlan: []; openForm: [] }>()
const actionEditorOpen = ref(false)
const actionDraft = ref<PathConfigConfiguredActionInput[]>([])

// actions 只保留当前节点的 F-008 动作行。
const savedActions = computed(() => props.node ? (props.draft.actionConfigurations[props.node.key] ?? pathConfigActionsInput(props.node)) : [])
const actions = computed(() => actionDraft.value)
const canAddAction = computed(() => Boolean(props.node?.actionConfiguration.catalog.some(item => item.enabled && !actions.value.some(action => action.kind === item.kind))) && actions.value.length < 10)
// cycleInputs 只回传服务端派生的循环事实。
// emitActions 复制动作行后更新父级草稿，避免 Vue Proxy 进入请求体。
function emitActions(next: PathConfigConfiguredActionInput[]) { if (props.node) emit('updateActionConfiguration', props.node.key, copyPathConfigActions(next)) }
// openActionEditor 打开弹窗时复制父级草稿，取消不会修改节点面板。
function openActionEditor() { actionDraft.value = copyPathConfigActions(savedActions.value); actionEditorOpen.value = true }
// saveActionEditor 只提交已经确认的弹窗草稿，页面保存按钮负责后端持久化。
function saveActionEditor() { emitActions(actionDraft.value); actionEditorOpen.value = false }
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
// actionOptions 过滤其他行已经使用的动作，避免同一动作被重复编排。
function actionOptions(index: number) { return (props.node?.actionConfiguration.catalog ?? []).filter(item => item.enabled && (item.kind === actions.value[index]?.kind || !actions.value.some((action, actionIndex) => actionIndex !== index && action.kind === item.kind))).map(item => ({ label: item.label, value: item.kind })) }
// updateAction 只更新弹窗草稿，避免用户未确认时污染节点面板。
function updateAction(index: number, patch: Partial<PathConfigConfiguredActionInput>) { const next = copyPathConfigActions(actions.value); const current = next[index]; if (!current) return; const kind = (patch.kind ?? current.kind) as PathConfigActionKind; if (next.some((action, actionIndex) => actionIndex !== index && action.kind === kind)) return; const definition = actionDefinition(kind); if (!definition) return; current.kind = kind; current.count = normalizedActionCount(patch.count ?? current.count); current.person = definition.requiresPerson ? (patch.person ?? current.person ?? (definition.person ? personDraft(definition.person) : undefined)) : undefined; actionDraft.value = next }
// updateActionPerson 只更新弹窗草稿中的独立人员策略。
function updateActionPerson(index: number, person: PathConfigPerson, patch: Partial<PathConfigPersonStrategyInput>) { const next = copyPathConfigActions(actions.value); if (!next[index]) return; const updated = { ...(next[index].person ?? personDraft(person)), ...patch, key: person.key }; updated.selected = resolvedPersonStrategySelection(person, updated); next[index].person = updated; actionDraft.value = next }
// addAction 只添加尚未使用的动作，默认不创建任何额外动作。
function addAction() { const definition = props.node?.actionConfiguration.catalog.find(item => item.enabled && !actions.value.some(action => action.kind === item.kind)); if (!definition || actions.value.length >= 10) return; actionDraft.value = [...actions.value, { key: `action-local-${Date.now()}`, kind: definition.kind as PathConfigActionKind, count: 1, person: definition.person ? personDraft(definition.person) : undefined }] }
// moveAction 调整动作对应的真实再次到达顺序。
function moveAction(index: number, offset: number) { const target = index + offset; if (target < 0 || target >= actions.value.length) return; const next = copyPathConfigActions(actions.value); [next[index], next[target]] = [next[target], next[index]]; actionDraft.value = next }
// removeAction 删除一个动作行。
function removeAction(index: number) { const next = copyPathConfigActions(actions.value); next.splice(index, 1); actionDraft.value = next }
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
              <n-select :value="personDraft(person).strategy" :options="strategyOptions(person)" @update:value="value => updatePersonStrategy(person, { strategy: value })" />
              <n-select v-if="personDraft(person).strategy === 'manual'" :multiple="person.multiple" :value="person.multiple ? personDraft(person).selected : (personDraft(person).selected[0] ?? null)" :options="personOptions(person)" @update:value="value => updatePersonStrategy(person, { selected: Array.isArray(value) ? value : (value ? [value] : []) })" />
            </div>
          </template>
          <p v-else>{{ person.detail }}</p>
          <small v-if="itemCount(person)">已解析 {{ itemCount(person) }} 项</small>
          <small v-if="person.note">{{ pathConfigurationMessage(person.note) }}</small>
        </div>
      </section>

      <section class="node-configuration-panel__section">
        <h3>准备情况</h3>
        <div v-if="savedActions.length" class="action-summary">
          <n-tag v-for="action in savedActions" :key="action.key" size="small">
            {{ actionDefinition(action.kind)?.label || action.kind }}
          </n-tag>
        </div>
        <span v-else class="muted-text">未添加额外动作</span>
        <n-space>
          <n-button type="primary" :disabled="node.lineBlocked || !node.actionConfiguration.catalog.length" @click="openActionEditor">动作配置</n-button>
        </n-space>
      </section>
    </div>

    <footer class="node-configuration-panel__footer">
      <n-alert v-if="saveError" type="error" :show-icon="false">{{ pathConfigurationMessage(saveError) }}</n-alert>
      <span v-else-if="missingCount">还有 {{ missingCount }} 项未满足配置要求</span>
      <n-button type="primary" :loading="saving" :disabled="saveDisabled" @click="emit('save')">保存当前节点</n-button>
    </footer>

    <n-modal v-model:show="actionEditorOpen">
      <n-card title="动作配置" style="width: min(680px, 94vw)">
        <div v-for="(action, index) in actions" :key="action.key" class="action-row">
          <strong class="action-arrival">第 {{ index + 1 }} 次</strong>
          <n-select class="action-select" :value="action.kind" :options="actionOptions(index)" @update:value="value => updateAction(index, { kind: value as PathConfigActionKind })" />
          <n-input-number class="action-count" :value="action.count" :min="1" :max="10" @update:value="value => updateAction(index, { count: Number(value) || 1 })" />
          <div class="action-row__actions">
            <n-button quaternary circle title="上移动作" aria-label="上移动作" :disabled="index === 0" @click="moveAction(index, -1)"><ArrowUpOutline /></n-button>
            <n-button quaternary circle title="下移动作" aria-label="下移动作" :disabled="index === actions.length - 1" @click="moveAction(index, 1)"><ArrowDownOutline /></n-button>
            <n-popconfirm @positive-click="removeAction(index)">
              <template #trigger><n-button type="error" secondary size="small" title="删除动作" aria-label="删除动作"><CloseOutline /> 删除</n-button></template>
              删除这个动作配置？
            </n-popconfirm>
          </div>
          <div v-if="actionPerson(action.kind)" class="action-person-fields">
            <n-select :value="action.person?.strategy || actionPerson(action.kind)!.strategy" :options="strategyOptions(actionPerson(action.kind)!)" @update:value="value => updateActionPerson(index, actionPerson(action.kind)!, { strategy: value as PathConfigPersonStrategyInput['strategy'] })" />
            <n-select v-if="(action.person?.strategy || actionPerson(action.kind)!.strategy) === 'manual'" :multiple="actionPerson(action.kind)!.multiple" :value="actionPerson(action.kind)!.multiple ? (action.person?.selected || []) : (action.person?.selected?.[0] || null)" :options="personOptions(actionPerson(action.kind)!)" @update:value="value => updateActionPerson(index, actionPerson(action.kind)!, { selected: Array.isArray(value) ? value : (value ? [value] : []) })" />
          </div>
        </div>
        <template #footer>
          <n-space justify="end"><n-button @click="actionEditorOpen = false">取消</n-button><n-button :disabled="!canAddAction" @click="addAction"><AddOutline /> 添加动作</n-button><n-button type="primary" @click="saveActionEditor">保存动作配置</n-button></n-space>
        </template>
      </n-card>
    </n-modal>

    <n-empty v-if="!node.persons.length && !node.actionConfiguration.catalog.length" description="此节点没有需要配置的内容" />
  </section>
</template>

<style scoped>
.node-configuration-panel{height:100%;display:flex;flex-direction:column;gap:12px;padding:16px}.node-configuration-panel__header,.node-configuration-panel__footer,.cycle-list li{display:flex;align-items:center;justify-content:space-between;gap:10px}.node-configuration-panel__body{overflow:auto;display:flex;flex-direction:column;gap:16px}.node-configuration-panel__section{border-top:1px solid #e5e7eb;padding-top:14px}.person-row{display:flex;flex-direction:column;align-items:stretch;gap:7px;margin-top:12px}.person-controls,.action-person-fields{display:flex;flex-direction:column;gap:7px}.action-summary{display:flex;flex-wrap:wrap;gap:6px;margin:4px 0 12px}.action-row{display:grid;grid-template-columns:88px minmax(220px,1fr) 78px 112px;gap:10px;align-items:center;padding:10px 0;border-bottom:1px solid #edf0f3}.action-arrival{white-space:nowrap;color:#475569}.action-row__actions{display:flex;justify-content:flex-end;gap:2px}.action-count{width:78px}.action-person-fields{grid-column:2 / -1;max-width:420px}.muted-text,.node-configuration-panel p,.node-configuration-panel small{color:#64748b}.node-configuration-panel__footer{border-top:1px solid #e5e7eb;padding-top:12px}.cycle-list{padding-left:0;list-style:none}.node-configuration-panel h2,.node-configuration-panel h3{margin:0}.node-configuration-panel p{margin:0}@media (max-width:680px){.node-configuration-panel{padding:12px}.action-row{grid-template-columns:1fr 78px 96px}.action-arrival{grid-column:1 / -1}.action-select{grid-column:1 / -1}.action-person-fields{grid-column:1 / -1}.action-row__actions{grid-column:3}.person-row{gap:6px}}
/* 操作列固定留出删除文字，避免删除入口只剩不可见的窄图标。 */
.action-row{grid-template-columns:88px minmax(220px,1fr) 78px 148px}
.action-row__actions{align-items:center;gap:4px}
@media (max-width:680px){.action-row__actions{grid-column:1 / -1;justify-content:flex-start}}
</style>
