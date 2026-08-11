<script setup lang="ts">
import {
  NAlert,
  NButton,
  NCard,
  NEmpty,
  NInput,
  NInputNumber,
  NModal,
  NPopover,
  NScrollbar,
  NSelect,
  NTag,
} from 'naive-ui'
import type { SelectOption } from 'naive-ui'
import { computed, ref, watch } from 'vue'

import { normalizedPersonStrategy, resolvedPersonStrategySelection, summarizePathConfigPersonItems } from './logic'
import type {
  PathConfigActionCatalogItem,
  PathConfigActionKind,
  PathConfigArrivalInput,
  PathConfigDraft,
  PathConfigNode,
  PathConfigPerson,
  PathConfigPersonStrategyInput,
} from './types'

const PERSON_PREVIEW_LIMIT = 3
const MAX_SAFE_PERSON_SEED = Number.MAX_SAFE_INTEGER
const personDetailsOpen = ref(false)
const detailedPerson = ref<PathConfigPerson | null>(null)
const activeVisit = ref(1)

const props = defineProps<{
  node: PathConfigNode | null
  warnings: string[]
  draft: PathConfigDraft
  saving: boolean
  saveDisabled: boolean
  missingCount: number
  saveError: string
  saveDetails: Array<{ kind: string, name: string, reason: string }>
  savedSuccessfully: boolean
  formComplete: boolean
}>()

const emit = defineEmits<{
  updatePersonStrategy: [person: PathConfigPerson, value: PathConfigPersonStrategyInput]
  updateArrivals: [nodeKey: string, value: PathConfigArrivalInput[]]
  save: []
  backToPlan: []
  openForm: []
}>()

const primaryPerson = computed(() => props.node?.persons.find(person => person.editable) ?? null)

// primaryPersonForAction 仅在模板已确认存在可编辑人员时取值，避免动作参数绕过人员范围。
function primaryPersonForAction(): PathConfigPerson {
  return primaryPerson.value as PathConfigPerson
}

watch(() => props.node?.key, () => { activeVisit.value = 1 })

// personOptions 转为可搜索的 Naive UI 不透明候选，页面不接触目标业务 ID。
function personOptions(person: PathConfigPerson): SelectOption[] {
  return person.options.map(option => ({ label: option.label, value: option.value }))
}

// strategyOptions 把策略目录转换成 Naive UI 可识别的选项结构。
function strategyOptions(person: PathConfigPerson): SelectOption[] {
  return person.strategies.map(option => ({ label: option.label, value: option.value }))
}

// rollbackOptions 把不透明回退目标转换成 Naive UI 选项结构。
function rollbackOptions(): SelectOption[] {
  return (props.node?.actionPlan.rollbackTargets ?? []).map(option => ({ label: option.label, value: option.value }))
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

// currentArrivals 返回当前节点到达计划的深复制，所有修改通过单一事件交给父页面持有。
function currentArrivals(): PathConfigArrivalInput[] {
  if (!props.node) return []
  return structuredClone(props.draft.arrivals[props.node.key] ?? [])
}

// actionOptions 返回当前节点可静态证明合法的动作目录。
function actionOptions(): SelectOption[] {
  return (props.node?.actionPlan.catalog ?? []).map(item => ({ label: item.label, value: item.kind }))
}

// actionDefinition 查找动作参数定义；未知动作不会获得输入控件。
function actionDefinition(kind: PathConfigActionKind): PathConfigActionCatalogItem | undefined {
  return props.node?.actionPlan.catalog.find(item => item.kind === kind)
}

// emitArrivals 统一重排连续访问序号并限制当前节点最大到达次数。
function emitArrivals(arrivals: PathConfigArrivalInput[]) {
  if (!props.node) return
  const limited = arrivals.slice(0, props.node.actionPlan.maxArrivals).map((arrival, index) => ({ ...arrival, visit: index + 1 }))
  activeVisit.value = Math.min(Math.max(1, activeVisit.value), Math.max(1, limited.length))
  emit('updateArrivals', props.node.key, limited)
}

// updateStep 合并一个动作步骤；切换动作时清除不适用参数，避免隐藏旧值误提交。
function updateStep(arrivalIndex: number, stepIndex: number, patch: Partial<PathConfigArrivalInput['steps'][number]>) {
  const arrivals = currentArrivals()
  const current = arrivals[arrivalIndex]?.steps[stepIndex]
  if (!current) return
  const next = { ...current, ...patch }
  const definition = actionDefinition(next.kind)
  if (!definition?.allowsOpinion) next.opinion = ''
  if (!definition?.requiresTarget) next.target = ''
  if (definition?.requiresPerson) {
    const primary = props.node?.persons.find(person => person.editable)
    if (primary && !next.person) next.person = personDraft(primary)
  }
  else delete next.person
  arrivals[arrivalIndex].steps[stepIndex] = next
  emitArrivals(arrivals)
}

// moveStep 在同一次到达内调整动作执行顺序，不跨访问移动。
function moveStep(arrivalIndex: number, stepIndex: number, offset: number) {
  const arrivals = currentArrivals()
  const steps = arrivals[arrivalIndex]?.steps
  const target = stepIndex + offset
  if (!steps || target < 0 || target >= steps.length) return
  ;[steps[stepIndex], steps[target]] = [steps[target], steps[stepIndex]]
  emitArrivals(arrivals)
}

// removeStep 删除单个动作；每次到达至少保留一个动作输入位置。
function removeStep(arrivalIndex: number, stepIndex: number) {
  const arrivals = currentArrivals()
  if (!arrivals[arrivalIndex] || arrivals[arrivalIndex].steps.length <= 1) return
  arrivals[arrivalIndex].steps.splice(stepIndex, 1)
  emitArrivals(arrivals)
}

// supplementaryAction 返回可排在终止动作前的动作；提交、审批结果、回退和暂存不能重复插入。
function supplementaryAction(): PathConfigActionCatalogItem | undefined {
  return props.node?.actionPlan.catalog.find(item => !terminalAction(item.kind))
}

// totalActionSteps 统计当前路径草稿动作总数，前端即时遵守服务端一百步上限。
function totalActionSteps(): number {
  return Object.values(props.draft.arrivals).reduce((total, arrivals) => total + arrivals.reduce((count, arrival) => count + arrival.steps.length, 0), 0)
}

// canAddStep 判断当前节点是否存在合法前置动作且整条路径仍有动作容量。
function canAddStep(): boolean {
  return Boolean(supplementaryAction()) && totalActionSteps() < (props.node?.actionPlan.maxPathSteps ?? 0)
}

// addStep 在当前到达的终止动作前增加合法前置步骤，不复制终止动作。
function addStep(arrivalIndex: number) {
  const arrivals = currentArrivals()
  const first = supplementaryAction()
  if (!first || !arrivals[arrivalIndex]) return
  const steps = arrivals[arrivalIndex].steps
  const insertAt = terminalAction(steps[steps.length - 1]?.kind) ? steps.length - 1 : steps.length
  steps.splice(insertAt, 0, { kind: first.kind, opinion: '', target: '' })
  emitArrivals(arrivals)
}

// addArrival 新增连续到达并复制上一次计划，让回退循环配置保持紧凑可理解。
function addArrival(copyPrevious = true) {
  const arrivals = currentArrivals()
  if (!props.node || arrivals.length >= props.node.actionPlan.maxArrivals) return
  const source = copyPrevious && arrivals.length ? structuredClone(arrivals[arrivals.length - 1].steps) : []
  const first = props.node.actionPlan.catalog[0]
	if (!first || totalActionSteps() + Math.max(1, source.length) > props.node.actionPlan.maxPathSteps) return
  arrivals.push({ visit: arrivals.length + 1, steps: source.length ? source : [{ kind: first.kind, opinion: '', target: '' }] })
  emitArrivals(arrivals)
  activeVisit.value = arrivals.length
}

// removeLastArrival 只允许从末尾删除，保证到达序号连续且语义明确。
function removeLastArrival() {
  const arrivals = currentArrivals()
  if (arrivals.length <= 1) return
  arrivals.pop()
  emitArrivals(arrivals)
}

// updateStepPerson 更新加签或移交的受限人员策略，候选范围与当前节点主人员规则相同。
function updateStepPerson(arrivalIndex: number, stepIndex: number, person: PathConfigPerson, patch: Partial<PathConfigPersonStrategyInput>) {
  const arrivals = currentArrivals()
  const step = arrivals[arrivalIndex]?.steps[stepIndex]
  if (!step) return
  const base = step.person ?? personDraft(person)
  const next = { ...base, ...patch, key: person.key }
  next.selected = resolvedPersonStrategySelection(person, next)
  step.person = next
  emitArrivals(arrivals)
}

// stepPersonDraft 返回加签或移交的当前人员策略；首次配置复用节点主人员规则作为安全起点。
function stepPersonDraft(step: PathConfigArrivalInput['steps'][number], person: PathConfigPerson): PathConfigPersonStrategyInput {
  return step.person ?? personDraft(person)
}

// terminalAction 判断会结束单次到达的动作，新增前置动作必须插在它之前。
function terminalAction(kind: PathConfigActionKind): boolean {
  return ['submit', 'approve_pass', 'reject_no_pass', 'draft_save', 'rollback_previous', 'transfer_approver'].includes(kind)
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
              <n-button quaternary circle size="small" aria-label="查看模板要求" title="查看模板要求">i</n-button>
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
          <n-tag size="small" :bordered="false" :type="node.status === 'configured' ? 'success' : node.status === 'affected' ? 'error' : node.status === 'runtime' ? 'info' : 'warning'">
            {{ node.statusName }}
          </n-tag>
        </div>
      </header>

      <div class="node-configuration-panel__scroll">
        <n-alert v-for="(warning, index) in warnings" :key="`warning-${index}`" type="warning" :show-icon="false" size="small">{{ warning }}</n-alert>
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
            <p v-if="person.note">{{ person.note }}</p>
          </div>
        </section>

        <section v-if="node.actionPlan.catalog.length" class="node-configuration-panel__section" aria-labelledby="node-actions-heading">
          <div class="node-configuration-panel__section-heading">
            <h3 id="node-actions-heading">动作计划</h3>
            <span>最多 {{ node.actionPlan.maxArrivals }} 次到达</span>
          </div>
          <n-alert v-if="node.actionPlan.note" type="warning" :show-icon="false" size="small">{{ node.actionPlan.note }}</n-alert>
          <div class="node-configuration-panel__visits" aria-label="节点到达次数">
            <n-button v-for="arrival in (draft.arrivals[node.key] ?? [])" :key="arrival.visit" size="tiny" :type="activeVisit === arrival.visit ? 'primary' : 'default'" @click="activeVisit = arrival.visit">
              第 {{ arrival.visit }} 次
            </n-button>
            <n-button size="tiny" :disabled="(draft.arrivals[node.key]?.length ?? 0) >= node.actionPlan.maxArrivals" @click="addArrival(true)">复制前一次</n-button>
            <n-button size="tiny" :disabled="(draft.arrivals[node.key]?.length ?? 0) <= 1" @click="removeLastArrival">删除末次</n-button>
          </div>
          <template v-for="(arrival, arrivalIndex) in (draft.arrivals[node.key] ?? [])" :key="arrival.visit">
            <div v-if="activeVisit === arrival.visit" class="node-configuration-panel__arrival">
              <div v-for="(step, stepIndex) in arrival.steps" :key="`${arrival.visit}-${stepIndex}`" class="node-configuration-panel__step">
                <div class="node-configuration-panel__step-head">
                  <strong>步骤 {{ stepIndex + 1 }}</strong>
                  <div>
                    <n-button quaternary circle size="tiny" title="上移" aria-label="上移动作" :disabled="stepIndex === 0" @click="moveStep(arrivalIndex, stepIndex, -1)">↑</n-button>
                    <n-button quaternary circle size="tiny" title="下移" aria-label="下移动作" :disabled="stepIndex === arrival.steps.length - 1" @click="moveStep(arrivalIndex, stepIndex, 1)">↓</n-button>
                    <n-button quaternary circle size="tiny" title="删除" aria-label="删除动作" :disabled="arrival.steps.length <= 1" @click="removeStep(arrivalIndex, stepIndex)">×</n-button>
                  </div>
                </div>
                <n-select :value="step.kind" :options="actionOptions()" aria-label="动作类型" @update:value="value => updateStep(arrivalIndex, stepIndex, { kind: value })" />
                <small>{{ actionDefinition(step.kind)?.description }}</small>
                <n-input v-if="actionDefinition(step.kind)?.allowsOpinion" :value="step.opinion" type="textarea" :autosize="{ minRows: 2, maxRows: 4 }" maxlength="1000" show-count placeholder="可选处理意见" @update:value="value => updateStep(arrivalIndex, stepIndex, { opinion: value })" />
                <n-select v-if="actionDefinition(step.kind)?.requiresTarget" :value="step.target || null" :options="rollbackOptions()" placeholder="选择更早业务节点" @update:value="value => updateStep(arrivalIndex, stepIndex, { target: value || '' })" />
                <template v-if="actionDefinition(step.kind)?.requiresPerson && primaryPerson">
                  <span class="node-configuration-panel__parameter-title">{{ step.kind === 'add_sign' ? '加签人员' : step.kind === 'transfer_approver' ? '移交人员' : '处理人员' }}</span>
                  <n-select
                    :value="stepPersonDraft(step, primaryPersonForAction()).strategy"
                    :options="strategyOptions(primaryPersonForAction())"
                    @update:value="value => updateStepPerson(arrivalIndex, stepIndex, primaryPersonForAction(), { strategy: value })"
                  />
                  <n-select
                    v-if="stepPersonDraft(step, primaryPersonForAction()).strategy === 'manual'"
                    :multiple="primaryPersonForAction().multiple"
                    filterable
                    :value="primaryPersonForAction().multiple ? stepPersonDraft(step, primaryPersonForAction()).selected : (stepPersonDraft(step, primaryPersonForAction()).selected[0] ?? null)"
                    :options="personOptions(primaryPersonForAction())"
                    @update:value="value => updateStepPerson(arrivalIndex, stepIndex, primaryPersonForAction(), { selected: Array.isArray(value) ? value : (value ? [value] : []) })"
                  />
                  <n-input-number
                    v-if="stepPersonDraft(step, primaryPersonForAction()).strategy === 'random'"
                    :value="stepPersonDraft(step, primaryPersonForAction()).seed"
                    :min="1"
                    :max="MAX_SAFE_PERSON_SEED"
                    aria-label="动作人员随机种子"
                    @update:value="value => updateStepPerson(arrivalIndex, stepIndex, primaryPersonForAction(), { seed: value || 1 })"
                  />
                  <div class="node-configuration-panel__resolved-persons">
                    <span>最终使用</span>
                    <n-tag v-for="name in selectedPersonNames(primaryPersonForAction(), stepPersonDraft(step, primaryPersonForAction())).slice(0, PERSON_PREVIEW_LIMIT)" :key="name" size="small" :bordered="false" type="success">{{ name }}</n-tag>
                  </div>
                </template>
              </div>
              <n-button dashed block size="small" :disabled="!canAddStep()" @click="addStep(arrivalIndex)">添加动作步骤</n-button>
            </div>
          </template>
        </section>

        <n-empty v-if="!node.persons.length && !node.actionPlan.catalog.length" size="small" description="此节点没有需要配置的内容" />
      </div>

      <footer class="node-configuration-panel__footer">
        <n-alert v-if="saveError" type="error" :show-icon="false" size="small">
          {{ saveError }}
          <ul v-if="saveDetails.length"><li v-for="(item, index) in saveDetails" :key="index">{{ item.name }}：{{ item.reason }}</li></ul>
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

      <n-modal :show="personDetailsOpen" :mask-closable="true" @update:show="show => { if (!show) closePersonDetails() }">
        <n-card v-if="detailedPerson" class="node-configuration-panel__person-modal" :bordered="false" role="dialog" aria-modal="true" aria-labelledby="person-details-title">
          <template #header><div class="node-configuration-panel__person-modal-heading"><span>人员与范围详情</span><h3 id="person-details-title">{{ detailedPerson.title }}</h3></div></template>
          <template #header-extra><n-button quaternary size="small" aria-label="关闭人员详情" @click="closePersonDetails">关闭</n-button></template>
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
.node-configuration-panel__header { border-bottom: 1px solid var(--flow-edge-color); }
.node-configuration-panel__title-row, .node-configuration-panel__section-heading, .node-configuration-panel__step-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 10px; }
.node-configuration-panel__header h2 { margin: 3px 0 8px; font-size: 17px; line-height: 1.35; }
.node-configuration-panel__eyebrow { font-size: 12px; opacity: .7; }
.node-configuration-panel__tags, .node-configuration-panel__footer-actions, .node-configuration-panel__visits, .node-configuration-panel__resolved-persons { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
.node-configuration-panel__scroll { min-height: 0; padding: 12px 14px; overflow-y: auto; overscroll-behavior: contain; scrollbar-gutter: stable; }
.node-configuration-panel__section + .node-configuration-panel__section { margin-top: 18px; }
.node-configuration-panel__section h3 { margin: 0 0 9px; font-size: 14px; }
.node-configuration-panel__section-heading > span, .node-configuration-panel__step small, .node-configuration-panel__parameter-title { font-size: 12px; opacity: .7; }
.node-configuration-panel__requirements-popover ul, .node-configuration-panel__person-items, .node-configuration-panel__person-modal ul { display: grid; gap: 7px; padding: 0; margin: 8px 0 0; list-style: none; }
.node-configuration-panel__requirements-popover li { display: grid; gap: 2px; padding-left: 8px; border-left: 2px solid var(--flow-edge-color); }
.node-configuration-panel__requirements-popover small { line-height: 1.45; opacity: .72; }
.node-configuration-panel__person, .node-configuration-panel__arrival { display: grid; gap: 8px; margin-bottom: 12px; }
.node-configuration-panel__person label { display: flex; align-items: center; gap: 6px; font-size: 13px; }
.node-configuration-panel__readonly { padding: 7px 9px; margin: 0; font-size: 12px; line-height: 1.5; background: color-mix(in srgb, var(--flow-edge-color) 18%, transparent); border-radius: 4px; }
.node-configuration-panel__resolved-persons > span { font-size: 12px; opacity: .7; }
.node-configuration-panel__person-items li, .node-configuration-panel__person-modal li { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 7px; min-width: 0; font-size: 12px; }
.node-configuration-panel__person-items li > span, .node-configuration-panel__person-modal li > span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.node-configuration-panel__person-more { justify-self: start; }
.node-configuration-panel__step { display: grid; gap: 7px; padding: 9px; border: 1px solid var(--flow-edge-color); border-radius: 4px; }
.node-configuration-panel__step-head > div { display: flex; gap: 2px; }
.node-configuration-panel__person-modal { width: min(520px, calc(100vw - 32px)); max-height: min(520px, calc(100dvh - 48px)); }
.node-configuration-panel__person-modal h3 { margin: 3px 0 0; font-size: 16px; }
.node-configuration-panel__person-modal-heading > span { font-size: 12px; opacity: .7; }
.node-configuration-panel__person-modal-scroll { min-height: 0; padding: 12px 4px 4px 0; }
.node-configuration-panel__footer { display: grid; gap: 10px; border-top: 1px solid var(--flow-edge-color); font-size: 12px; }
.node-configuration-panel__footer-actions { justify-content: flex-end; }
.node-configuration-panel__footer ul { margin: 5px 0 0; padding-left: 18px; }
.node-configuration-panel__empty { align-self: center; padding: 24px; }
</style>
