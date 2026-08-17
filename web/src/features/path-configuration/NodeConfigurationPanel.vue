<script setup lang="ts">
import { NAlert, NButton, NCard, NEmpty, NInputNumber, NModal, NPopconfirm, NSelect, NSpace, NTag } from 'naive-ui'
import { computed, ref } from 'vue'
import { AddOutline, ArrowDownOutline, ArrowUpOutline, CloseOutline } from '@vicons/ionicons5'
import { copyPathConfigActions, normalizedActionCount, normalizedPersonStrategy, pathConfigActionsInput, pathConfigurationMessage, pathConfigurationStatusName, resolvedPersonStrategySelection, summarizePathConfigPersonItems } from './logic'
import type { PathConfigActionCycle, PathConfigActionCycleInput, PathConfigActionKind, PathConfigConfiguredActionInput, PathConfigDraft, PathConfigNode, PathConfigPerson, PathConfigPersonStrategyInput } from './types'

const MAX_SAFE_PERSON_SEED = Number.MAX_SAFE_INTEGER
const props = defineProps<{ node: PathConfigNode | null; draft: PathConfigDraft; saving: boolean; saveDisabled: boolean; missingCount: number; saveError: string; saveDetails: Array<{ kind: string; name: string; reason: string }>; savedSuccessfully: boolean; formComplete: boolean; actionCycles: PathConfigActionCycle[] }>()
const emit = defineEmits<{ updatePersonStrategy: [person: PathConfigPerson, value: PathConfigPersonStrategyInput]; updateActionConfiguration: [nodeKey: string, value: PathConfigConfiguredActionInput[]]; updateActionCycles: [value: PathConfigActionCycleInput[]]; save: []; backToPlan: []; openForm: [] }>()
const actionEditorOpen = ref(false)
const cycleEditorOpen = ref(false)
const cycleType = ref<PathConfigActionCycleInput['type']>('restart_from_initiator')

// actions 只保留当前节点的 F-008 动作行。
const actions = computed(() => props.node ? (props.draft.actionConfigurations[props.node.key] ?? pathConfigActionsInput(props.node)) : [])
// cycleInputs 只回传服务端派生的循环事实。
const cycleInputs = computed(() => props.actionCycles.map(cycle => ({ key: cycle.key, type: cycle.type, endNodeKey: cycle.endNodeKey, count: 1 as const })))
// emitActions 复制动作行后更新父级草稿，避免 Vue Proxy 进入请求体。
function emitActions(next: PathConfigConfiguredActionInput[]) { if (props.node) emit('updateActionConfiguration', props.node.key, copyPathConfigActions(next)) }
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
// updateAction 更新单行的类型、次数和人员参数。
function updateAction(index: number, patch: Partial<PathConfigConfiguredActionInput>) { const next = copyPathConfigActions(actions.value); const current = next[index]; if (!current) return; const kind = (patch.kind ?? current.kind) as PathConfigActionKind; const definition = actionDefinition(kind); if (!definition) return; current.kind = kind; current.count = normalizedActionCount(patch.count ?? current.count); current.person = definition.requiresPerson ? (patch.person ?? current.person ?? (definition.person ? personDraft(definition.person) : undefined)) : undefined; emitActions(next) }
// updateActionPerson 更新动作的独立人员策略。
function updateActionPerson(index: number, person: PathConfigPerson, patch: Partial<PathConfigPersonStrategyInput>) { const next = copyPathConfigActions(actions.value); if (!next[index]) return; const updated = { ...(next[index].person ?? personDraft(person)), ...patch, key: person.key }; updated.selected = resolvedPersonStrategySelection(person, updated); next[index].person = updated; emitActions(next) }
// addAction 追加当前目录中的第一个安全动作；不会自动创建循环。
function addAction() { const definition = props.node?.actionConfiguration.catalog.find(item => item.enabled); if (!definition || actions.value.length >= 10) return; emitActions([...actions.value, { key: `action-local-${Date.now()}`, kind: definition.kind as PathConfigActionKind, count: 1, person: definition.person ? personDraft(definition.person) : undefined }]) }
// moveAction 调整动作对应的真实再次到达顺序。
function moveAction(index: number, offset: number) { const target = index + offset; if (target < 0 || target >= actions.value.length) return; const next = copyPathConfigActions(actions.value); [next[index], next[target]] = [next[target], next[index]]; emitActions(next) }
// removeAction 删除一个动作行。
function removeAction(index: number) { const next = copyPathConfigActions(actions.value); next.splice(index, 1); emitActions(next) }
// addCycle 以当前节点作为引擎派生终点，不允许传入成员或回退目标。
function addCycle() { if (!props.node) return; emit('updateActionCycles', [...cycleInputs.value, { key: `cycle-local-${Date.now()}`, type: cycleType.value, endNodeKey: props.node.key, count: 1 }]); cycleEditorOpen.value = false }
// removeCycle 删除循环草稿，不调用目标平台。
function removeCycle(key: string) { emit('updateActionCycles', cycleInputs.value.filter(cycle => cycle.key !== key)) }
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
            <n-select :value="personDraft(person).strategy" :options="strategyOptions(person)" @update:value="value => updatePersonStrategy(person, { strategy: value })" />
            <n-input-number v-if="personDraft(person).strategy === 'random'" :value="personDraft(person).seed" :min="1" :max="MAX_SAFE_PERSON_SEED" @update:value="value => updatePersonStrategy(person, { seed: value || 1 })" />
            <n-select v-if="personDraft(person).strategy === 'manual'" :multiple="person.multiple" :value="person.multiple ? personDraft(person).selected : (personDraft(person).selected[0] ?? null)" :options="personOptions(person)" @update:value="value => updatePersonStrategy(person, { selected: Array.isArray(value) ? value : (value ? [value] : []) })" />
          </template>
          <p v-else>{{ person.detail }}</p>
          <small v-if="itemCount(person)">已解析 {{ itemCount(person) }} 项</small>
          <small v-if="person.note">{{ pathConfigurationMessage(person.note) }}</small>
        </div>
      </section>

      <section class="node-configuration-panel__section">
        <h3>准备情况</h3>
        <p>流程第几次走到这个节点，就执行第几行。第 2 次走到这里，才会用第 2 行；一次到达只执行一行。</p>
        <p v-if="node.actionConfiguration.base">基础动作：{{ node.actionConfiguration.base.label }} 1 次（系统默认）</p>
        <p v-if="actions.length">已配置 {{ actions.length }} 个可选动作</p>
        <p v-else>尚未配置动作。</p>
        <n-space>
          <n-button type="primary" :disabled="node.lineBlocked || !node.actionConfiguration.catalog.length" @click="actionEditorOpen = true">动作配置</n-button>
          <n-button :disabled="node.lineBlocked" @click="cycleEditorOpen = true">循环配置</n-button>
        </n-space>
        <ul v-if="actionCycles.length" class="cycle-list">
          <li v-for="cycle in actionCycles" :key="cycle.key">
            <span>{{ cycle.label }}：{{ cycle.members.join(' → ') }}</span>
            <n-popconfirm @positive-click="removeCycle(cycle.key)">
              <template #trigger><n-button text title="删除循环"><CloseOutline /></n-button></template>
              删除这个循环配置？
            </n-popconfirm>
          </li>
        </ul>
      </section>
    </div>

    <footer class="node-configuration-panel__footer">
      <n-alert v-if="saveError" type="error" :show-icon="false">{{ pathConfigurationMessage(saveError) }}</n-alert>
      <span v-else-if="missingCount">还有 {{ missingCount }} 项未满足配置要求</span>
      <span v-else>保存只更新当前节点的人员、动作和循环</span>
      <n-button type="primary" :loading="saving" :disabled="saveDisabled" @click="emit('save')">保存当前节点</n-button>
    </footer>

    <n-modal v-model:show="actionEditorOpen">
      <n-card title="动作配置" style="width: min(680px, 94vw)">
        <p class="beginner-hint">流程第几次走到这个节点，就执行第几行；一次到达只执行一行。</p>
        <n-alert type="info" :show-icon="false">一次到达只执行一行。</n-alert>
        <div v-if="node.actionConfiguration.base" class="action-base-row"><span>系统默认</span><strong>{{ node.actionConfiguration.base.label }}</strong><span>固定 1 次</span></div>
        <div v-for="(action, index) in actions" :key="action.key" class="action-row">
          <div class="action-row__header">
            <strong>第 {{ index + 1 }} 次到达</strong>
            <n-space>
              <n-button text title="上移动作" :disabled="index === 0" @click="moveAction(index, -1)"><ArrowUpOutline /></n-button>
              <n-button text title="下移动作" :disabled="index === actions.length - 1" @click="moveAction(index, 1)"><ArrowDownOutline /></n-button>
              <n-popconfirm @positive-click="removeAction(index)">
                <template #trigger><n-button text title="删除动作"><CloseOutline /></n-button></template>
                删除这个动作配置？
              </n-popconfirm>
            </n-space>
          </div>
          <n-select :value="action.kind" :options="node.actionConfiguration.catalog.map(item => ({ label: item.label, value: item.kind }))" @update:value="value => updateAction(index, { kind: value as PathConfigActionKind })" />
          <n-input-number :value="action.count" :min="1" :max="10" @update:value="value => updateAction(index, { count: Number(value) || 1 })" />
          <template v-if="actionPerson(action.kind)">
            <n-select :value="action.person?.strategy || actionPerson(action.kind)!.strategy" :options="strategyOptions(actionPerson(action.kind)!)" @update:value="value => updateActionPerson(index, actionPerson(action.kind)!, { strategy: value as PathConfigPersonStrategyInput['strategy'] })" />
            <n-select v-if="(action.person?.strategy || actionPerson(action.kind)!.strategy) === 'manual'" :multiple="actionPerson(action.kind)!.multiple" :value="actionPerson(action.kind)!.multiple ? (action.person?.selected || []) : (action.person?.selected?.[0] || null)" :options="personOptions(actionPerson(action.kind)!)" @update:value="value => updateActionPerson(index, actionPerson(action.kind)!, { selected: Array.isArray(value) ? value : (value ? [value] : []) })" />
          </template>
        </div>
        <template #footer>
          <n-space justify="end"><n-button @click="actionEditorOpen = false">取消</n-button><n-button :disabled="actions.length >= 10" @click="addAction"><AddOutline /> 添加动作</n-button><n-button type="primary" @click="emit('save')">保存动作配置</n-button></n-space>
        </template>
      </n-card>
    </n-modal>

    <n-modal v-model:show="cycleEditorOpen">
      <n-card title="循环配置" style="width: min(560px, 94vw)">
        <n-select v-model:value="cycleType" :options="[{ label: '不同意后重新提交', value: 'restart_from_initiator' }, { label: '回退上一步后重做', value: 'redo_previous_task' }]" />
        <p class="cycle-fixed-note">每个循环固定执行一轮。</p>
        <n-alert type="info" :show-icon="false">重新提交会从发起人开始重新解析条件、并行和人员；回退只能由引擎返回真实上一个待办。</n-alert>
        <template #footer><n-space justify="end"><n-button @click="cycleEditorOpen = false">取消</n-button><n-button type="primary" @click="addCycle">加入循环</n-button></n-space></template>
      </n-card>
    </n-modal>

    <n-empty v-if="!node.persons.length && !node.actionConfiguration.catalog.length" description="此节点没有需要配置的内容" />
  </section>
</template>

<style scoped>
.node-configuration-panel{height:100%;display:flex;flex-direction:column;gap:12px}.node-configuration-panel__header,.node-configuration-panel__footer,.action-row__header,.cycle-list li{display:flex;align-items:center;justify-content:space-between;gap:10px}.node-configuration-panel__body{overflow:auto;display:flex;flex-direction:column;gap:12px}.node-configuration-panel__section{border-top:1px solid #e5e7eb;padding-top:10px}.person-row{display:grid;grid-template-columns:120px minmax(0,1fr);gap:8px;margin-top:8px;align-items:center}.action-row{display:grid;grid-template-columns:120px minmax(0,1fr) 88px auto;gap:8px;align-items:center;padding:8px 0;border-bottom:1px solid #edf0f3}.action-row__header{display:contents}.action-base-row{display:grid;grid-template-columns:120px 1fr 80px;gap:8px;align-items:center;padding:8px 0;color:#64748b;border-bottom:1px solid #edf0f3}.node-configuration-panel__footer{border-top:1px solid #e5e7eb;padding-top:10px}.cycle-list{padding-left:0;list-style:none}.node-configuration-panel h2,.node-configuration-panel h3{margin:0}.node-configuration-panel p,.node-configuration-panel small{color:#64748b}@media (max-width:680px){.person-row,.action-row,.action-base-row{grid-template-columns:1fr}}
</style>
