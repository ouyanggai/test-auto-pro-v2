<script setup lang="ts">
import {
  NAlert,
  NButton,
  NCard,
  NEmpty,
  NInputNumber,
  NIcon,
  NModal,
  NPopover,
  NScrollbar,
  NSelect,
  NTag,
} from 'naive-ui'
import { AddOutline, ArrowDownOutline, ArrowUpOutline, CloseOutline, InformationCircleOutline } from '@vicons/ionicons5'
import type { SelectOption } from 'naive-ui'
import { computed, ref } from 'vue'

import { copyPathConfigActionPlan, normalizedPersonStrategy, pathConfigActionPlanInput, pathConfigurationMessage, pathConfigurationStatusName, resolvedPersonStrategySelection, summarizePathConfigPersonItems } from './logic'
import type {
  PathConfigActionCatalogItem,
  PathConfigActionCycle,
  PathConfigActionCycleInput,
  PathConfigActionKind,
  PathConfigActionPlanInput,
  PathConfigDraft,
  PathConfigNode,
  PathConfigPerson,
  PathConfigPersonStrategy,
  PathConfigPersonStrategyInput,
} from './types'

const PERSON_PREVIEW_LIMIT = 3
const MAX_SAFE_PERSON_SEED = Number.MAX_SAFE_INTEGER
const personDetailsOpen = ref(false)
const detailedPerson = ref<PathConfigPerson | null>(null)
const actionEditorOpen = ref(false)
const cycleEditorOpen = ref(false)
const cycleType = ref<PathConfigActionCycleInput['type']>('restart_from_initiator')
const cycleCount = ref(1)

const props = defineProps<{
  node: PathConfigNode | null
  draft: PathConfigDraft
  saving: boolean
  saveDisabled: boolean
  missingCount: number
  saveError: string
  saveDetails: Array<{ kind: string, name: string, reason: string }>
  savedSuccessfully: boolean
  formComplete: boolean
  actionCycles: PathConfigActionCycle[]
}>()

const emit = defineEmits<{
  updatePersonStrategy: [person: PathConfigPerson, value: PathConfigPersonStrategyInput]
  updateActionPlan: [nodeKey: string, value: PathConfigActionPlanInput]
  updateActionCycles: [value: PathConfigActionCycleInput[]]
  save: []
  backToPlan: []
  openForm: []
}>()

// cycleInputs 保留服务端派生的终点不透明键，仅用于下一次保存，不向用户展示技术标识。
const cycleInputs = computed<PathConfigActionCycleInput[]>(() => props.actionCycles.map(cycle => ({ key: cycle.key, type: cycle.type, endNodeKey: cycle.endNodeKey, count: cycle.count })))

// addCycle 只允许以当前节点作为引擎派生终点；节点成员与回退前驱由服务端核对。
function addCycle() {
  if (!props.node) return
  const next = [...cycleInputs.value]
  next.push({ key: `cycle-local-${Date.now()}`, type: cycleType.value, endNodeKey: props.node.key, count: Math.max(1, Math.min(10, cycleCount.value || 1)) })
  emit('updateActionCycles', next)
  cycleEditorOpen.value = false
}

// removeCycle 删除一条已保存或草稿循环；真正保存仍需用户明确点击当前节点保存。
function removeCycle(key: string) {
  emit('updateActionCycles', cycleInputs.value.filter(cycle => cycle.key !== key))
}

// personOptions 转为可搜索的 Naive UI 不透明候选，页面不接触目标业务 ID。
function personOptions(person: PathConfigPerson): SelectOption[] {
  return person.options.map(option => ({ label: option.label, value: option.value }))
}

// strategyOptions 把策略目录转换成 Naive UI 可识别的选项结构。
function strategyOptions(person: PathConfigPerson): SelectOption[] {
  return person.strategies.map(option => ({ label: option.label, value: option.value }))
}

// personDraft 返回当前人员策略草稿；缺失时从服务端权威投影初始化。
function personDraft(person: PathConfigPerson): PathConfigPersonStrategyInput {
  return normalizedPersonStrategy(person, props.draft.personStrategies[person.key])
}

// selectedPersonNames 把最终不透明候选映射为具体中文名称。
function selectedPersonNames(person: PathConfigPerson, input = personDraft(person)): string[] {
  const labels = new Map(person.options.map(option => [option.value, option.label]))
  return resolvedPersonStrategySelection(person, input).map(value => labels.get(value)).filter((value): value is string => Boolean(value))
}

// updatePersonStrategy 合并人员策略草稿，非手动策略的最终名单由同一纯规则重新计算。
function updatePersonStrategy(person: PathConfigPerson, patch: Partial<PathConfigPersonStrategyInput>) {
  const next = { ...personDraft(person), ...patch, key: person.key }
  next.selected = resolvedPersonStrategySelection(person, next)
  emit('updatePersonStrategy', person, next)
}

// personItemSummary 收敛侧栏预览和弹窗总数，避免大量模板对象把固定侧栏无限撑长。
function personItemSummary(person: PathConfigPerson) {
  return summarizePathConfigPersonItems(person.items, PERSON_PREVIEW_LIMIT)
}

// openPersonDetails 打开当前人员规则的完整只读名称列表。
function openPersonDetails(person: PathConfigPerson) {
  detailedPerson.value = person
  personDetailsOpen.value = true
}

// closePersonDetails 关闭弹窗并释放上一个节点的展示引用。
function closePersonDetails() {
  personDetailsOpen.value = false
  detailedPerson.value = null
}

const actionPlan = computed(() => props.node ? (props.draft.actionPlans[props.node.key] ?? pathConfigActionPlanInput(props.node)) : null)
const resultKind = computed<PathConfigActionKind>(() => actionPlan.value?.result.kind ?? (props.node?.kind === 'start' ? 'submit' : 'approve_pass'))

// resultPerson 返回当前处理结果的人员草稿，模板只通过函数读取以避开空节点切换期。
function resultPerson(): PathConfigPersonStrategyInput | undefined {
  return actionPlan.value?.result.person
}

// selectedResultPersonNames 把移交处理结果的人员候选映射为中文名称。
function selectedResultPersonNames(kind: PathConfigActionKind): string[] {
  const person = actionPerson(kind)
  if (!person) return []
  return selectedPersonNames(person, resultPerson())
}

// addSignDisabledReason 返回加签动作不可添加时的目录原因，避免模板对可选定义做非空断言。
function addSignDisabledReason(): string {
  return actionDefinition('add_sign')?.disabledReason || '当前节点不支持添加加签动作'
}

// resultOptions 展示当前节点完整处理结果目录；静态不可用项保留但不能选择。
function resultOptions(): SelectOption[] {
  if (!props.node) return []
  return props.node.actionPlan.catalog
    .filter(item => item.kind !== 'add_sign' && (props.node?.kind === 'start' ? item.kind === 'submit' : item.kind !== 'submit'))
    .map(item => ({ label: item.label, value: item.kind, disabled: !item.enabled }))
}

// actionDefinition 查找动作参数定义；未知动作不会获得输入控件。
function actionDefinition(kind: PathConfigActionKind): PathConfigActionCatalogItem | undefined {
  return props.node?.actionPlan.catalog.find(item => item.kind === kind)
}

// actionPerson 返回动作目录自己的合法人员范围，加签节点处理人不再依赖当前节点主处理人是否可编辑。
function actionPerson(kind: PathConfigActionKind): PathConfigPerson | null {
  return actionDefinition(kind)?.person ?? null
}

// requiredActionPerson 供 requiresPerson 模板分支取得服务端已保证存在的动作人员规则。
function requiredActionPerson(kind: PathConfigActionKind): PathConfigPerson {
  return actionPerson(kind) as PathConfigPerson
}

// actionRuleReason 返回静态禁用原因，运行态条件只保留在集中说明中。
function actionRuleReason(item: PathConfigActionCatalogItem): string {
  return item.enabled ? '' : item.disabledReason
}

// actionRuleStatus 为信息弹层提供简短静态状态。
function actionRuleStatus(item: PathConfigActionCatalogItem): string {
  return item.enabled ? '可用' : '不可用'
}

// emitPlan 把语义化动作草稿复制成普通对象后交给父页面，避免直接修改响应式值。
function emitPlan(next: PathConfigActionPlanInput) {
  if (!props.node) return
  emit('updateActionPlan', props.node.key, copyPathConfigActionPlan(next))
}

// updateResult 切换唯一处理结果，并只保留该结果实际需要的回退或移交参数。
function updateResult(kind: PathConfigActionKind) {
  const definition = actionDefinition(kind)
  if (!definition?.enabled || !props.node || !actionPlan.value || kind === 'add_sign') return
  const next = copyPathConfigActionPlan(actionPlan.value)
  next.result = {
    kind,
    opinion: '',
    target: definition.requiresTarget && props.node.actionPlan.rollbackTargets.length === 1 ? props.node.actionPlan.rollbackTargets[0].value : '',
    person: definition.person ? personDraft(definition.person) : undefined,
  }
  emitPlan(next)
}

// restoreDefaultResult 恢复默认同意：同意不进入界面，仅作为未配置其他处理结果时的默认动作。
function restoreDefaultResult() {
  if (actionDefinition('approve_pass')?.enabled) updateResult('approve_pass')
}

// resultOverridden 判断用户是否显式选择了非默认处理结果。
const resultOverridden = computed<boolean>(() => {
  const kind = actionPlan.value?.result.kind ?? ''
  return kind !== '' && kind !== 'approve_pass' && kind !== 'submit'
})

// addSignTotalCount 统计当前全部加签次数，界面与保存前共同遵守单节点十次上限。
const addSignTotalCount = computed(() => (actionPlan.value?.addSignNodes ?? []).reduce((sum, item) => sum + Math.max(1, Math.min(10, Number(item.count) || 1)), 0))

// addSignNode 新增一个拥有独立人员策略的审批节点；次数表示按目标语义创建的加签节点数量。
function addSignNode() {
  const definition = actionDefinition('add_sign')
  if (!definition?.enabled || !definition.person || !actionPlan.value || addSignTotalCount.value >= 10) return
  const next = copyPathConfigActionPlan(actionPlan.value)
  next.addSignNodes.push({ person: personDraft(definition.person), count: 1 })
  emitPlan(next)
}

// updateAddSignCount 调整单个加签行的次数，总次数不能超过单节点十次上限。
function updateAddSignCount(index: number, count: number | null) {
  if (!actionPlan.value) return
  const next = copyPathConfigActionPlan(actionPlan.value)
  const row = next.addSignNodes[index]
  if (!row) return
  const normalized = Math.max(1, Math.min(10, Number(count) || 1))
  if (addSignTotalCount.value - Math.max(1, Math.min(10, Number(row.count) || 1)) + normalized > 10) return
  row.count = normalized
  emitPlan(next)
}

// moveAddSignNode 调整加签节点的执行顺序，唯一处理结果始终位于列表之后。
function moveAddSignNode(index: number, offset: number) {
  if (!actionPlan.value) return
  const next = copyPathConfigActionPlan(actionPlan.value)
  const target = index + offset
  if (target < 0 || target >= next.addSignNodes.length) return
  ;[next.addSignNodes[index], next.addSignNodes[target]] = [next.addSignNodes[target], next.addSignNodes[index]]
  emitPlan(next)
}

// removeAddSignNode 删除一个加签节点，不影响当前节点唯一处理结果。
function removeAddSignNode(index: number) {
  if (!actionPlan.value) return
  const next = copyPathConfigActionPlan(actionPlan.value)
  next.addSignNodes.splice(index, 1)
  emitPlan(next)
}

// updateAddSignPerson 更新一个加签节点自己的人员策略，不能共享或覆盖其他加签节点。
function updateAddSignPerson(index: number, person: PathConfigPerson, patch: Partial<PathConfigPersonStrategyInput>) {
  if (!actionPlan.value) return
  const next = copyPathConfigActionPlan(actionPlan.value)
  const base = next.addSignNodes[index]?.person ?? personDraft(person)
  const updated = { ...base, ...patch, key: person.key }
  updated.selected = resolvedPersonStrategySelection(person, updated)
  next.addSignNodes[index] = { person: updated, count: Math.max(1, Math.min(10, Number(next.addSignNodes[index]?.count) || 1)) }
  emitPlan(next)
}

// updateResultPerson 更新移交结果自己的人员策略，候选范围与服务端目录一致。
function updateResultPerson(person: PathConfigPerson, patch: Partial<PathConfigPersonStrategyInput>) {
  if (!actionPlan.value) return
  const plan = copyPathConfigActionPlan(actionPlan.value)
  const base = plan.result.person ?? personDraft(person)
  const updated = { ...base, ...patch, key: person.key }
  updated.selected = resolvedPersonStrategySelection(person, updated)
  plan.result.person = updated
  emitPlan(plan)
}

// actionEditorOptions 只允许选择当前节点目录中已静态证明可用的动作，避免编辑器制造无效类型。
const actionEditorOptions = computed<SelectOption[]>(() => (props.node?.actionPlan.catalog ?? [])
  .filter(item => item.enabled)
  .map(item => ({ label: item.label, value: item.kind })))

// configuredActionDefinition 从当前目录获取动作参数要求，目录变化后不会沿用旧节点的人员范围。
function configuredActionDefinition(kind: PathConfigActionKind) {
  return props.node?.actionPlan.catalog.find(item => item.kind === kind && item.enabled)
}

// configuredActionPerson 返回当前动作目录的专用候选范围，缺失时不渲染人员输入。
function configuredActionPerson(kind: PathConfigActionKind): PathConfigPerson | undefined {
  return configuredActionDefinition(kind)?.person
}

// rollbackOptions 转为 Naive UI 选项，回退目标始终由当前路径的后端目录提供。
function rollbackOptions(): SelectOption[] {
  return (props.node?.actionPlan.rollbackTargets ?? []).map(item => ({ label: item.label, value: item.value }))
}

// configuredActionSelected 把选择控件回传统一为不透明人员键数组。
function configuredActionSelected(value: unknown): string[] {
  if (Array.isArray(value)) return value.map(String)
  return value ? [String(value)] : []
}

// updateCombinationCount 更新组合循环次数；它只影响配置期序列化，不与任一动作的 Count 混用。
function updateCombinationCount(value: number | null) {
  if (!actionPlan.value) return
  const next = copyPathConfigActionPlan(actionPlan.value)
  next.combinationCount = Math.max(1, Math.min(10, Number(value) || 1))
  emitPlan(next)
}

// updateConfiguredAction 更新一个配置期动作行；改变类型时立即重置不再适用的目标与人员参数。
function updateConfiguredAction(index: number, patch: Partial<PathConfigActionPlanInput['actions'][number]>) {
  if (!actionPlan.value) return
  const next = copyPathConfigActionPlan(actionPlan.value)
  const action = next.actions[index]
  if (!action) return
  const kind = (patch.kind ?? action.kind) as PathConfigActionKind
  const definition = configuredActionDefinition(kind)
  if (!definition) return
  action.kind = kind
  action.count = Math.max(1, Math.min(10, Number(patch.count ?? action.count) || 1))
  action.target = definition.requiresTarget ? (patch.target ?? action.target ?? '') : ''
  action.person = definition.requiresPerson
    ? (patch.person ?? action.person ?? personDraft(definition.person!))
    : undefined
  emitPlan(next)
}

// addConfiguredAction 追加一个可用动作并用目录默认值初始化，实际执行仍不属于 F-007。
function addConfiguredAction() {
  const definition = props.node?.actionPlan.catalog.find(item => item.enabled)
  if (!definition || !actionPlan.value || actionPlan.value.actions.length >= 10) return
  const next = copyPathConfigActionPlan(actionPlan.value)
  next.actions.push({ key: `action-local-${Date.now()}-${next.actions.length}`, kind: definition.kind as PathConfigActionKind, count: 1, target: definition.requiresTarget && props.node!.actionPlan.rollbackTargets.length === 1 ? props.node!.actionPlan.rollbackTargets[0].value : '', person: definition.requiresPerson && definition.person ? personDraft(definition.person) : undefined })
  emitPlan(next)
}

// moveConfiguredAction 只调整配置期组合顺序，不修改兼容动作步骤或触发任何目标写入。
function moveConfiguredAction(index: number, offset: number) {
  if (!actionPlan.value) return
  const next = copyPathConfigActionPlan(actionPlan.value)
  const target = index + offset
  if (target < 0 || target >= next.actions.length) return
  ;[next.actions[index], next.actions[target]] = [next.actions[target], next.actions[index]]
  emitPlan(next)
}

// removeConfiguredAction 删除一个配置期动作行；节点保存仍由后端按最新目录再次校验。
function removeConfiguredAction(index: number) {
  if (!actionPlan.value) return
  const next = copyPathConfigActionPlan(actionPlan.value)
  next.actions.splice(index, 1)
  emitPlan(next)
}
</script>

<template>
  <section class="node-configuration-panel">
    <template v-if="node">
      <header class="node-configuration-panel__header">
        <div class="node-configuration-panel__title-row">
          <div>
            <span class="node-configuration-panel__eyebrow">当前节点</span>
            <h2>{{ node.name }}</h2>
          </div>
          <n-popover trigger="click" placement="bottom-end" :width="300" scrollable>
            <template #trigger>
              <n-button quaternary circle size="small" aria-label="查看模板要求" title="查看模板要求"><n-icon><InformationCircleOutline /></n-icon></n-button>
            </template>
            <div class="node-configuration-panel__requirements-popover">
              <strong>模板要求</strong>
              <ul v-if="node.requirements.length">
                <li v-for="(requirement, index) in node.requirements" :key="index">
                  <span>{{ requirement.title }}</span>
                  <small>{{ requirement.detail }}</small>
                </li>
              </ul>
              <n-empty v-else size="small" description="当前节点没有额外模板要求" />
            </div>
          </n-popover>
        </div>
        <div class="node-configuration-panel__tags">
          <n-tag size="small" :bordered="false">{{ node.typeName }}</n-tag>
          <n-tag size="small" :bordered="false" :type="node.status === 'configured' || node.status === 'not_required' ? 'success' : node.status === 'runtime' ? 'info' : 'warning'">
            {{ pathConfigurationStatusName(node.status) }}
          </n-tag>
        </div>
      </header>

      <div class="node-configuration-panel__scroll">
        <n-alert v-if="node.lineBlocked" type="warning" :show-icon="false" size="small">前序动作已结束当前线路，本节点无需继续配置。</n-alert>

        <section v-if="node.persons.length" class="node-configuration-panel__section" aria-labelledby="node-persons-heading">
          <h3 id="node-persons-heading">处理人员</h3>
          <div v-for="(person, index) in node.persons" :key="`person-${index}`" class="node-configuration-panel__person">
            <label>
              <span>{{ person.title }}</span>
              <n-tag v-if="person.required" size="tiny" :bordered="false" type="error">必选</n-tag>
              <n-tag v-if="person.mode === 'runtime'" size="tiny" :bordered="false" type="info">运行时确定</n-tag>
            </label>
            <template v-if="person.editable">
              <n-select
                :value="personDraft(person).strategy"
                :options="strategyOptions(person)"
                :disabled="node.lineBlocked"
                aria-label="人员策略"
                @update:value="value => updatePersonStrategy(person, { strategy: value })"
              />
              <n-input-number
                v-if="personDraft(person).strategy === 'random'"
                :value="personDraft(person).seed"
                :min="1"
                :max="MAX_SAFE_PERSON_SEED"
                :show-button="true"
                aria-label="随机种子"
                @update:value="value => updatePersonStrategy(person, { seed: value || 1 })"
              />
              <n-select
                v-if="personDraft(person).strategy === 'manual'"
                :multiple="person.multiple"
                filterable
                :value="person.multiple ? personDraft(person).selected : (personDraft(person).selected[0] ?? null)"
                :options="personOptions(person)"
                :placeholder="person.minCount > 1 ? `至少选择 ${person.minCount} 人` : '请选择处理人'"
                @update:value="value => updatePersonStrategy(person, { selected: Array.isArray(value) ? value : (value ? [value] : []) })"
              />
              <div class="node-configuration-panel__resolved-persons">
                <span>最终使用</span>
                <n-tag v-for="name in selectedPersonNames(person).slice(0, PERSON_PREVIEW_LIMIT)" :key="name" size="small" :bordered="false" type="success">{{ name }}</n-tag>
                <small v-if="!selectedPersonNames(person).length">当前策略未得到合法人员</small>
                <n-button v-else-if="selectedPersonNames(person).length > PERSON_PREVIEW_LIMIT" text type="primary" size="tiny" @click="openPersonDetails(person)">查看全部 {{ selectedPersonNames(person).length }} 人</n-button>
              </div>
            </template>
            <p v-else class="node-configuration-panel__readonly">{{ person.detail }}</p>
            <ul v-if="person.items.length" class="node-configuration-panel__person-items" aria-label="目标模板已配置对象">
              <li v-for="(item, itemIndex) in personItemSummary(person).preview" :key="`${item.category}-${item.name}-${itemIndex}`">
                <n-tag size="tiny" :bordered="false">{{ item.category }}</n-tag>
                <span>{{ item.name }}</span>
                <small v-if="item.count > 1">{{ item.count }} 项</small>
              </li>
            </ul>
            <n-button v-if="personItemSummary(person).total > PERSON_PREVIEW_LIMIT" text type="primary" size="small" class="node-configuration-panel__person-more" @click="openPersonDetails(person)">
              查看全部 {{ personItemSummary(person).total }} 项
            </n-button>
            <p v-if="person.note">{{ pathConfigurationMessage(person.note) }}</p>
          </div>
        </section>

        <section class="node-configuration-panel__section" aria-labelledby="node-actions-heading">
          <h3 id="node-actions-heading">动作与循环</h3>
          <p v-if="actionPlan?.actions.length" class="node-configuration-panel__readonly">已配置 {{ actionPlan.actions.length }} 个动作；次数只在节点真实再次到达时按顺序使用。</p>
          <p v-else class="node-configuration-panel__readonly">尚未配置动作。</p>
          <div class="node-configuration-panel__commands">
            <n-button type="primary" size="small" :disabled="node.lineBlocked || !node.actionPlan.catalog.some(item => item.enabled)" @click="actionEditorOpen = true">动作配置</n-button>
            <n-button size="small" :disabled="node.lineBlocked" @click="cycleEditorOpen = true">循环配置</n-button>
          </div>
          <ul v-if="actionCycles.length" class="node-configuration-panel__cycle-summary">
            <li v-for="cycle in actionCycles" :key="cycle.key"><strong>{{ cycle.label }} × {{ cycle.count }}</strong><span>{{ cycle.members.join(' → ') }}</span><n-button quaternary circle size="tiny" title="删除循环" aria-label="删除循环" @click="removeCycle(cycle.key)"><n-icon><CloseOutline /></n-icon></n-button></li>
          </ul>
        </section>

        <n-empty v-if="!node.persons.length && !node.actionPlan.catalog.length" size="small" description="此节点没有需要配置的内容" />
      </div>

      <footer class="node-configuration-panel__footer">
        <n-alert v-if="saveError" type="error" :show-icon="false" size="small">
          {{ pathConfigurationMessage(saveError) }}
          <ul v-if="saveDetails.length"><li v-for="(item, index) in saveDetails" :key="index">{{ item.name }}：{{ pathConfigurationMessage(item.reason) }}</li></ul>
        </n-alert>
        <n-alert v-else-if="savedSuccessfully" type="success" :show-icon="false" size="small">{{ formComplete ? '当前路径的节点与表单配置均已完成' : '当前节点配置已保存，节点配置已完成' }}</n-alert>
        <span v-else-if="missingCount">还有 {{ missingCount }} 项未满足模板要求</span>
        <span v-else>保存只更新当前节点的人员策略与动作计划</span>
        <div class="node-configuration-panel__footer-actions">
          <template v-if="savedSuccessfully">
            <n-button size="small" @click="emit('backToPlan')">返回计划详情</n-button>
            <n-button v-if="!formComplete" size="small" type="primary" @click="emit('openForm')">配置表单数据</n-button>
          </template>
          <n-button v-else type="primary" :loading="saving" :disabled="saveDisabled" @click="emit('save')">保存当前节点</n-button>
        </div>
      </footer>

      <n-modal :show="actionEditorOpen" :mask-closable="true" @update:show="show => actionEditorOpen = show">
        <n-card v-if="node && actionPlan" class="node-configuration-panel__action-modal" :bordered="false" role="dialog" aria-modal="true" aria-labelledby="action-editor-title">
          <template #header><div class="node-configuration-panel__action-modal-heading"><span>动作配置</span><h3 id="action-editor-title">{{ node.name }}</h3></div></template>
          <template #header-extra><n-button quaternary circle size="small" title="关闭" aria-label="关闭动作配置" @click="actionEditorOpen = false"><n-icon><CloseOutline /></n-icon></n-button></template>
          <n-alert v-if="!actionPlan.actions.length" type="info" :show-icon="false" size="small">当前还没有动作。次数表示后续真实再次到达时的顺序，不会连续调用目标接口。</n-alert>
          <n-scrollbar class="node-configuration-panel__action-modal-scroll" style="max-height: 420px">
            <div v-for="(action, index) in actionPlan.actions" :key="action.key || index" class="node-configuration-panel__action-modal-row">
              <div class="node-configuration-panel__action-modal-row-head">
                <strong>动作 {{ index + 1 }}</strong>
                <div class="node-configuration-panel__action-row-tools">
                  <n-button quaternary circle size="small" :disabled="index === 0" title="上移" aria-label="上移" @click="moveConfiguredAction(index, -1)"><n-icon><ArrowUpOutline /></n-icon></n-button>
                  <n-button quaternary circle size="small" :disabled="index === actionPlan.actions.length - 1" title="下移" aria-label="下移" @click="moveConfiguredAction(index, 1)"><n-icon><ArrowDownOutline /></n-icon></n-button>
                  <n-button quaternary circle size="small" title="删除动作" aria-label="删除动作" @click="removeConfiguredAction(index)"><n-icon><CloseOutline /></n-icon></n-button>
                </div>
              </div>
              <n-select :value="action.kind" :options="actionEditorOptions" aria-label="动作类型" @update:value="value => updateConfiguredAction(index, { kind: value as PathConfigActionKind })" />
              <n-input-number :value="action.count" :min="1" :max="10" :show-button="true" aria-label="动作次数" @update:value="value => updateConfiguredAction(index, { count: Number(value) || 1 })" />
              <p v-if="configuredActionDefinition(action.kind)?.requiresTarget" class="node-configuration-panel__readonly">回退目标由引擎按当前待办的真实上一节点决定。</p>
              <template v-for="person in [configuredActionPerson(action.kind)]" :key="person?.key || action.key">
                <template v-if="person">
                  <n-alert v-if="!person.options.length" type="warning" :show-icon="false" size="small">当前动作没有可用候选人员。</n-alert>
                  <n-select :value="action.person?.strategy || person.strategy" :options="strategyOptions(person)" aria-label="动作人员策略" @update:value="value => updateConfiguredAction(index, { person: { ...personDraft(person), ...action.person, key: person.key, strategy: value as PathConfigPersonStrategy } })" />
                  <n-input-number v-if="(action.person?.strategy || person.strategy) === 'random'" :value="action.person?.seed || person.strategySeed" :min="1" :max="MAX_SAFE_PERSON_SEED" :show-button="true" aria-label="动作随机种子" @update:value="value => updateConfiguredAction(index, { person: { ...personDraft(person), ...action.person, key: person.key, seed: Number(value) || 1 } })" />
                  <n-select v-if="(action.person?.strategy || person.strategy) === 'manual'" :multiple="person.multiple" filterable :value="person.multiple ? (action.person?.selected || []) : (action.person?.selected?.[0] || null)" :options="personOptions(person)" :placeholder="person.minCount > 1 ? `至少选择 ${person.minCount} 人` : '请选择处理人'" @update:value="value => updateConfiguredAction(index, { person: { ...personDraft(person), ...action.person, key: person.key, selected: configuredActionSelected(value) } })" />
                </template>
              </template>
            </div>
          </n-scrollbar>
          <template #footer><div class="node-configuration-panel__modal-footer"><n-button size="small" @click="actionEditorOpen = false">取消</n-button><n-button size="small" :disabled="actionPlan.actions.length >= 10" @click="addConfiguredAction"><n-icon><AddOutline /></n-icon>添加动作</n-button><n-button size="small" type="primary" :loading="saving" @click="emit('save')">保存动作配置</n-button></div></template>
        </n-card>
      </n-modal>

      <n-modal :show="cycleEditorOpen" :mask-closable="true" @update:show="show => cycleEditorOpen = show">
        <n-card v-if="node" class="node-configuration-panel__action-modal" :bordered="false" role="dialog" aria-modal="true" aria-labelledby="cycle-editor-title">
          <template #header><div class="node-configuration-panel__action-modal-heading"><span>循环配置</span><h3 id="cycle-editor-title">{{ node.name }}</h3></div></template>
          <n-select v-model:value="cycleType" :options="[{ label: '不同意后重新提交', value: 'restart_from_initiator' }, { label: '回退上一步后重做', value: 'redo_previous_task' }]" aria-label="循环方式" />
          <n-input-number v-model:value="cycleCount" :min="1" :max="10" :show-button="true" aria-label="循环次数" />
          <n-alert type="info" :show-icon="false" size="small">重新提交会从发起人开始重新解析条件、并行和人员；回退只能由引擎返回真实上一个待办。</n-alert>
          <template #footer><div class="node-configuration-panel__modal-footer"><n-button size="small" @click="cycleEditorOpen = false">取消</n-button><n-button size="small" type="primary" @click="addCycle">加入循环</n-button></div></template>
        </n-card>
      </n-modal>

      <n-modal :show="personDetailsOpen" :mask-closable="true" @update:show="show => { if (!show) closePersonDetails() }">
        <n-card v-if="detailedPerson" class="node-configuration-panel__person-modal" :bordered="false" role="dialog" aria-modal="true" aria-labelledby="person-details-title">
          <template #header><div class="node-configuration-panel__person-modal-heading"><span>人员与范围详情</span><h3 id="person-details-title">{{ detailedPerson.title }}</h3></div></template>
          <template #header-extra><n-button quaternary circle size="small" title="关闭" aria-label="关闭人员详情" @click="closePersonDetails"><n-icon><CloseOutline /></n-icon></n-button></template>
          <n-scrollbar class="node-configuration-panel__person-modal-scroll" style="max-height: 360px">
            <strong v-if="selectedPersonNames(detailedPerson).length">最终使用人员 · {{ selectedPersonNames(detailedPerson).length }} 人</strong>
            <ul v-if="selectedPersonNames(detailedPerson).length"><li v-for="(name, itemIndex) in selectedPersonNames(detailedPerson)" :key="`selected-${name}-${itemIndex}`"><n-tag size="small" :bordered="false" type="success">人员</n-tag><span>{{ name }}</span></li></ul>
            <strong v-if="detailedPerson.items.length">目标模板对象 · {{ personItemSummary(detailedPerson).total }} 项</strong>
            <ul v-if="detailedPerson.items.length"><li v-for="(item, itemIndex) in detailedPerson.items" :key="`${item.category}-${item.name}-${itemIndex}`"><n-tag size="small" :bordered="false">{{ item.category }}</n-tag><span>{{ item.name }}</span><small v-if="item.count > 1">对应 {{ item.count }} 项目标范围</small></li></ul>
          </n-scrollbar>
        </n-card>
      </n-modal>
    </template>
    <n-empty v-else class="node-configuration-panel__empty" description="请在当前路径上选择一个节点" />
  </section>
</template>

<style scoped>
.node-configuration-panel { display: grid; grid-template-rows: auto minmax(0, 1fr) auto; width: 100%; height: 100%; color: var(--flow-label-color); background: var(--flow-surface-color); }
.node-configuration-panel__header, .node-configuration-panel__footer { padding: 12px 14px; background: var(--flow-surface-color); }
.node-configuration-panel__action-modal { width: min(760px, calc(100vw - 32px)); }
.node-configuration-panel__action-modal-heading, .node-configuration-panel__action-modal-row-head, .node-configuration-panel__action-modal-loop { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.node-configuration-panel__action-modal-heading h3 { margin: 2px 0 0; font-size: 16px; }
.node-configuration-panel__action-modal-loop { margin-bottom: 12px; }
.node-configuration-panel__action-modal-loop .n-input-number { width: 150px; }
.node-configuration-panel__commands, .node-configuration-panel__modal-footer { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; margin-top: 10px; }
.node-configuration-panel__cycle-summary { display: grid; gap: 7px; padding: 0; margin: 10px 0 0; list-style: none; }
.node-configuration-panel__cycle-summary li { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 3px 8px; padding: 8px; border: 1px solid var(--flow-edge-color); border-radius: 4px; font-size: 12px; }
.node-configuration-panel__cycle-summary span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; opacity: .76; }
.node-configuration-panel__cycle-summary .n-button { grid-column: 2; grid-row: 1 / span 2; }
.node-configuration-panel__action-modal-row { display: grid; gap: 8px; padding: 10px 0; border-bottom: 1px solid var(--flow-edge-color); }
.node-configuration-panel__action-modal-row:last-child { border-bottom: 0; }
.node-configuration-panel__header { border-bottom: 1px solid var(--flow-edge-color); }
.node-configuration-panel__title-row, .node-configuration-panel__section-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 10px; }
.node-configuration-panel__header h2 { margin: 3px 0 8px; font-size: 17px; line-height: 1.35; }
.node-configuration-panel__eyebrow { font-size: 12px; opacity: .7; }
.node-configuration-panel__tags, .node-configuration-panel__footer-actions, .node-configuration-panel__resolved-persons { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
.node-configuration-panel__scroll { min-height: 0; padding: 12px 14px; overflow-y: auto; overscroll-behavior: contain; scrollbar-gutter: stable; }
.node-configuration-panel__section + .node-configuration-panel__section { margin-top: 18px; }
.node-configuration-panel__section h3 { margin: 0 0 9px; font-size: 14px; }
.node-configuration-panel__section-heading > span, .node-configuration-panel__parameter-title { font-size: 12px; opacity: .7; }
.node-configuration-panel__requirements-popover ul, .node-configuration-panel__person-items, .node-configuration-panel__person-modal ul { display: grid; gap: 7px; padding: 0; margin: 8px 0 0; list-style: none; }
.node-configuration-panel__requirements-popover li { display: grid; gap: 2px; padding-left: 8px; border-left: 2px solid var(--flow-edge-color); }
.node-configuration-panel__requirements-popover small { line-height: 1.45; opacity: .72; }
.node-configuration-panel__person { display: grid; gap: 8px; margin-bottom: 12px; }
.node-configuration-panel__person label { display: flex; align-items: center; gap: 6px; font-size: 13px; }
.node-configuration-panel__readonly { padding: 7px 9px; margin: 0; font-size: 12px; line-height: 1.5; background: color-mix(in srgb, var(--flow-edge-color) 18%, transparent); border-radius: 4px; }
.node-configuration-panel__resolved-persons > span { font-size: 12px; opacity: .7; }
.node-configuration-panel__person-items li, .node-configuration-panel__person-modal li { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 7px; min-width: 0; font-size: 12px; }
.node-configuration-panel__person-items li > span, .node-configuration-panel__person-modal li > span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.node-configuration-panel__person-more { justify-self: start; }
.node-configuration-panel__action-plan { display: grid; gap: 9px; }
.node-configuration-panel__action-row { display: grid; gap: 7px; padding: 10px; border: 1px solid var(--flow-edge-color); border-radius: 4px; }
.node-configuration-panel__action-row--result { border-style: dashed; }
.node-configuration-panel__action-row-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.node-configuration-panel__action-row-tools { display: flex; align-items: center; gap: 5px; }
.node-configuration-panel__action-row-selected { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; }
.node-configuration-panel__action-row-selected .node-configuration-panel__more,
.node-configuration-panel__row-empty { font-size: 12px; opacity: .68; }
.node-configuration-panel__action-add { display: flex; align-items: center; gap: 8px; }
.node-configuration-panel__person-picker { display: grid; gap: 8px; padding: 4px 2px; }
.node-configuration-panel__result-select { min-width: 188px; }
.node-configuration-panel__row-tools { display: flex; align-items: center; gap: 5px; }
.node-configuration-panel__action-info { color: var(--flow-direction-color); background: color-mix(in srgb, var(--flow-direction-color) 14%, var(--flow-surface-color)); border: 1px solid color-mix(in srgb, var(--flow-direction-color) 45%, var(--flow-edge-color)); }
.node-configuration-panel__action-info:hover, .node-configuration-panel__action-info:focus-visible { background: color-mix(in srgb, var(--flow-direction-color) 22%, var(--flow-surface-color)); }
.node-configuration-panel__action-rules { display: grid; gap: 8px; }
.node-configuration-panel__action-rules > strong, .node-configuration-panel__action-rules p { margin: 0; }
.node-configuration-panel__action-rules ul { display: grid; gap: 9px; padding: 0 6px 0 0; margin: 0; list-style: none; }
.node-configuration-panel__action-rules li { display: grid; gap: 3px; padding: 8px; background: color-mix(in srgb, var(--flow-edge-color) 13%, transparent); border-radius: 4px; }
.node-configuration-panel__action-rules li > div { display: flex; align-items: center; gap: 6px; }
.node-configuration-panel__action-rules li > div > small { margin-left: auto; opacity: .68; }
.node-configuration-panel__action-rules li > p, .node-configuration-panel__action-rules li > small { font-size: 12px; line-height: 1.45; }
.node-configuration-panel__runtime-note { margin: 0; font-size: 12px; line-height: 1.55; }
.node-configuration-panel__person-modal { width: min(520px, calc(100vw - 32px)); max-height: min(520px, calc(100dvh - 48px)); }
.node-configuration-panel__person-modal h3 { margin: 3px 0 0; font-size: 16px; }
.node-configuration-panel__person-modal-heading > span { font-size: 12px; opacity: .7; }
.node-configuration-panel__person-modal-scroll { min-height: 0; padding: 12px 4px 4px 0; }
.node-configuration-panel__footer { display: grid; gap: 10px; border-top: 1px solid var(--flow-edge-color); font-size: 12px; }
.node-configuration-panel__footer-actions { justify-content: flex-end; }
.node-configuration-panel__footer ul { margin: 5px 0 0; padding-left: 18px; }
.node-configuration-panel__empty { align-self: center; padding: 24px; }
</style>
