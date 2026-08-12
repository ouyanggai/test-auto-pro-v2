<script setup lang="ts">
import {
  NAlert,
  NButton,
  NCard,
  NEmpty,
  NInputNumber,
  NModal,
  NPopover,
  NScrollbar,
  NSelect,
  NTag,
} from 'naive-ui'
import type { SelectOption } from 'naive-ui'
import { computed, ref, watch } from 'vue'

import { canUsePathConfigAction, normalizedPathConfigActionCount, normalizedPersonStrategy, pathConfigActionRowsFromArrivals, pathConfigActionRowsToArrivals, resolvedPersonStrategySelection, summarizePathConfigPersonItems } from './logic'
import type {
  PathConfigActionCatalogItem,
  PathConfigActionKind,
  PathConfigActionRow,
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
const newActionKind = ref<PathConfigActionKind | null>(null)

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

watch(() => props.node?.key, () => {
  newActionKind.value = null
})

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

const actionRows = computed(() => props.node ? pathConfigActionRowsFromArrivals(props.draft.arrivals[props.node.key] ?? []) : [])

// actionOptions 返回完整动作目录，并阻止行内切换绕过当前动作次数上限。
function actionOptions(rowIndex: number): SelectOption[] {
  if (!props.node) return []
  return props.node.actionPlan.catalog.map(item => ({ label: item.label, value: item.kind, disabled: !canUsePathConfigAction(props.node as PathConfigNode, actionRows.value, item.kind, rowIndex) }))
}

// addActionOptions 在新增入口展示完整目录；静态不可用或达到次数上限的动作保持可见但禁用。
function addActionOptions(): SelectOption[] {
  if (!props.node) return []
  return props.node.actionPlan.catalog.map(item => ({ label: item.label, value: item.kind, disabled: !canAddAction(item.kind) }))
}

// actionDefinition 查找动作参数定义；未知动作不会获得输入控件。
function actionDefinition(kind: PathConfigActionKind): PathConfigActionCatalogItem | undefined {
  return props.node?.actionPlan.catalog.find(item => item.kind === kind)
}

// actionPerson 返回动作目录自己的合法人员范围，加签/移交不再依赖节点主处理人是否可编辑。
function actionPerson(kind: PathConfigActionKind): PathConfigPerson | null {
  return actionDefinition(kind)?.person ?? null
}

// requiredActionPerson 供 requiresPerson 模板分支取得服务端已保证存在的动作人员规则。
function requiredActionPerson(kind: PathConfigActionKind): PathConfigPerson {
  return actionPerson(kind) as PathConfigPerson
}

// emitRows 通过唯一纯函数展开兼容 arrivals，组件只呈现动作、次数和必要参数。
function emitRows(rows: PathConfigActionRow[]) {
  if (!props.node) return
  emit('updateArrivals', props.node.key, pathConfigActionRowsToArrivals(rows, props.node))
}

// editableRows 复制当前动作行及嵌套人员选择，避免直接修改 Vue 响应式草稿。
function editableRows(): PathConfigActionRow[] {
  return actionRows.value.map(row => ({ ...row, person: row.person ? { ...row.person, selected: [...row.person.selected] } : undefined }))
}

// configuredActionCount 汇总当前动作已配置次数，供信息弹层解释动态上限。
function configuredActionCount(kind: PathConfigActionKind): number {
  return actionRows.value.reduce((total, row) => row.kind === kind ? total + row.count : total, 0)
}

// actionRuleReason 返回静态禁用或当前配置已达上限的集中说明，行内不再重复铺开。
function actionRuleReason(item: PathConfigActionCatalogItem): string {
  if (!item.enabled) return item.disabledReason
  if (configuredActionCount(item.kind) >= item.maxCount) return `已达到当前配置上限（最多 ${item.maxCount} 次）`
  return ''
}

// actionRuleStatus 为信息弹层提供简短状态，达到配置上限与静态不可用分开表达。
function actionRuleStatus(item: PathConfigActionCatalogItem): string {
  if (!item.enabled) return '不可用'
  return configuredActionCount(item.kind) >= item.maxCount ? '已达上限' : '可用'
}

// canAddAction 同时执行单动作、当前任务结束动作和整条路径三层上限的即时门禁。
function canAddAction(kind: PathConfigActionKind): boolean {
  if (!props.node || !canUsePathConfigAction(props.node, actionRows.value, kind)) return false
  if (totalActionSteps() >= props.node.actionPlan.maxPathSteps) return false
  return !terminalAction(kind) || currentTerminalActionCount() < props.node.actionPlan.maxArrivals
}

// updateActionKind 切换动作时只保留适用参数；回退自动绑定唯一静态直接上一审批节点。
function updateActionKind(rowIndex: number, kind: PathConfigActionKind) {
  const definition = actionDefinition(kind)
  if (!definition?.enabled || !props.node || !canUsePathConfigAction(props.node, actionRows.value, kind, rowIndex)) return
  const rows = editableRows()
  const current = rows[rowIndex]
  if (!current) return
  rows[rowIndex] = {
    kind,
    count: normalizedPathConfigActionCount(props.node, kind, current.count),
    target: definition.requiresTarget && props.node?.actionPlan.rollbackTargets.length === 1 ? props.node.actionPlan.rollbackTargets[0].value : '',
    person: definition.person ? personDraft(definition.person) : undefined,
  }
  emitRows(rows)
}

// moveActionRow 调整动作执行顺序，终止组合合法性由即时校验和服务端共同保证。
function moveActionRow(rowIndex: number, offset: number) {
  const rows = editableRows()
  const target = rowIndex + offset
  if (target < 0 || target >= rows.length) return
  ;[rows[rowIndex], rows[target]] = [rows[target], rows[rowIndex]]
  emitRows(rows)
}

// removeActionRow 删除动作行；节点至少保留一个可执行动作。
function removeActionRow(rowIndex: number) {
  const rows = editableRows()
  if (rows.length <= 1) return
  rows.splice(rowIndex, 1)
  emitRows(rows)
}

// totalActionSteps 统计当前路径草稿动作总数，前端即时遵守服务端一百步上限。
function totalActionSteps(): number {
  return Object.values(props.draft.arrivals).reduce((total, arrivals) => total + arrivals.reduce((count, arrival) => count + arrival.steps.length, 0), 0)
}

// currentTerminalActionCount 统计会结束当前任务处理的动作次数，继续遵守兼容模型的十次上限。
function currentTerminalActionCount(): number {
  return actionRows.value.reduce((total, row) => total + (terminalAction(row.kind) ? row.count : 0), 0)
}

// addActionRow 新增动作行；加签自动排到首个终止动作之前，其他动作按列表顺序追加。
function addActionRow() {
  const kind = newActionKind.value
  const definition = kind ? actionDefinition(kind) : undefined
  if (!kind || !definition?.enabled || !props.node || !canAddAction(kind)) return
  const rows = editableRows()
  const row: PathConfigActionRow = {
    kind,
    count: 1,
    target: definition.requiresTarget && props.node.actionPlan.rollbackTargets.length === 1 ? props.node.actionPlan.rollbackTargets[0].value : '',
    person: definition.person ? personDraft(definition.person) : undefined,
  }
  const firstTerminal = rows.findIndex(item => terminalAction(item.kind))
  if (!terminalAction(kind) && firstTerminal >= 0) rows.splice(firstTerminal, 0, row)
  else rows.push(row)
  emitRows(rows)
  newActionKind.value = null
}

// updateActionRowCount 直接调整该动作执行次数，并同时遵守节点十次和路径一百步上限。
function updateActionRowCount(rowIndex: number, value: number | null) {
  if (!props.node || !value) return
  const rows = editableRows()
  const otherPathActions = totalActionSteps() - rows[rowIndex].count
  const terminalRoom = terminalAction(rows[rowIndex].kind) ? props.node.actionPlan.maxArrivals - (currentTerminalActionCount() - rows[rowIndex].count) : props.node.actionPlan.maxArrivals
  const actionRoom = normalizedPathConfigActionCount(props.node, rows[rowIndex].kind, value)
  rows[rowIndex].count = Math.max(1, Math.min(actionRoom, terminalRoom, props.node.actionPlan.maxPathSteps - otherPathActions))
  emitRows(rows)
}

// updateActionRowPerson 更新加签或移交的受限人员策略，候选范围与服务端目录完全一致。
function updateActionRowPerson(rowIndex: number, person: PathConfigPerson, patch: Partial<PathConfigPersonStrategyInput>) {
  const rows = editableRows()
  const row = rows[rowIndex]
  if (!row) return
  const base = row.person ?? personDraft(person)
  const next = { ...base, ...patch, key: person.key }
  next.selected = resolvedPersonStrategySelection(person, next)
  row.person = next
  emitRows(rows)
}

// terminalAction 判断会结束当前任务处理的动作，其他动作必须排在其之前。
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
            <n-popover trigger="click" placement="bottom-end" :width="340" scrollable>
              <template #trigger><n-button class="node-configuration-panel__action-info" circle size="small" aria-label="查看动作规则" title="查看动作规则">i</n-button></template>
              <div class="node-configuration-panel__action-rules">
                <strong>动作说明</strong>
                <p class="node-configuration-panel__runtime-note">次数表示计划真实执行次数，不是网络自动重试。每次执行前仍会核对实例、待办、审批权限、会签、并行及任务链状态。</p>
                <n-scrollbar style="max-height: 320px">
                  <ul>
                    <li v-for="item in node.actionPlan.catalog" :key="item.kind">
                      <div><strong>{{ item.label }}</strong><n-tag size="tiny" :bordered="false" :type="actionRuleReason(item) ? 'default' : 'success'">{{ actionRuleStatus(item) }}</n-tag><small>最多 {{ item.maxCount }} 次</small></div>
                      <p>{{ item.description }}</p>
                      <small v-if="actionRuleReason(item)">{{ actionRuleReason(item) }}</small>
                    </li>
                  </ul>
                </n-scrollbar>
              </div>
            </n-popover>
          </div>
          <n-alert v-if="node.actionPlan.note" type="warning" :show-icon="false" size="small">{{ node.actionPlan.note }}</n-alert>
          <div class="node-configuration-panel__action-rows">
            <div v-for="(row, rowIndex) in actionRows" :key="`${row.kind}-${rowIndex}`" class="node-configuration-panel__action-row">
              <div class="node-configuration-panel__action-main">
                <n-select class="node-configuration-panel__action-select" :value="row.kind" :options="actionOptions(rowIndex)" :consistent-menu-width="false" aria-label="动作" @update:value="value => updateActionKind(rowIndex, value)" />
                <label class="node-configuration-panel__row-count">
                  <span>次数</span>
                  <span v-if="row.kind === 'submit'" class="node-configuration-panel__fixed-count">固定 1 次</span>
                  <n-input-number v-else :value="row.count" :min="1" :max="actionDefinition(row.kind)?.maxCount ?? node.actionPlan.maxArrivals" size="small" aria-label="动作执行次数" @update:value="value => updateActionRowCount(rowIndex, value)" />
                </label>
                <div class="node-configuration-panel__row-tools">
                  <n-button quaternary circle size="tiny" title="上移" aria-label="上移动作" :disabled="rowIndex === 0" @click="moveActionRow(rowIndex, -1)">↑</n-button>
                  <n-button quaternary circle size="tiny" title="下移" aria-label="下移动作" :disabled="rowIndex === actionRows.length - 1" @click="moveActionRow(rowIndex, 1)">↓</n-button>
                  <n-button quaternary circle size="tiny" title="删除" aria-label="删除动作" :disabled="actionRows.length <= 1" @click="removeActionRow(rowIndex)">×</n-button>
                </div>
              </div>
              <p v-if="actionDefinition(row.kind)?.requiresTarget && node.actionPlan.rollbackTargets.length === 1" class="node-configuration-panel__readonly">回退至：{{ node.actionPlan.rollbackTargets[0].label }}</p>
              <template v-if="actionDefinition(row.kind)?.requiresPerson && actionPerson(row.kind)">
                  <span class="node-configuration-panel__parameter-title">{{ row.kind === 'add_sign' ? '加签人员' : '移交人员' }}</span>
                  <n-select
                    :value="row.person?.strategy ?? requiredActionPerson(row.kind).strategy"
                    :options="strategyOptions(requiredActionPerson(row.kind))"
                    @update:value="value => updateActionRowPerson(rowIndex, requiredActionPerson(row.kind), { strategy: value })"
                  />
                  <n-select
                    v-if="(row.person?.strategy ?? requiredActionPerson(row.kind).strategy) === 'manual'"
                    :multiple="requiredActionPerson(row.kind).multiple"
                    filterable
                    :value="requiredActionPerson(row.kind).multiple ? (row.person?.selected ?? []) : (row.person?.selected?.[0] ?? null)"
                    :options="personOptions(requiredActionPerson(row.kind))"
                    @update:value="value => updateActionRowPerson(rowIndex, requiredActionPerson(row.kind), { selected: Array.isArray(value) ? value : (value ? [value] : []) })"
                  />
                  <n-input-number
                    v-if="(row.person?.strategy ?? requiredActionPerson(row.kind).strategy) === 'random'"
                    :value="row.person?.seed ?? requiredActionPerson(row.kind).strategySeed"
                    :min="1"
                    :max="MAX_SAFE_PERSON_SEED"
                    aria-label="动作人员随机种子"
                    @update:value="value => updateActionRowPerson(rowIndex, requiredActionPerson(row.kind), { seed: value || 1 })"
                  />
                  <div class="node-configuration-panel__resolved-persons">
                    <span>最终使用</span>
                    <n-tag v-for="name in selectedPersonNames(requiredActionPerson(row.kind), row.person ?? personDraft(requiredActionPerson(row.kind))).slice(0, PERSON_PREVIEW_LIMIT)" :key="name" size="small" :bordered="false" type="success">{{ name }}</n-tag>
                  </div>
              </template>
            </div>
          </div>
          <div class="node-configuration-panel__add-action">
            <n-select v-model:value="newActionKind" class="node-configuration-panel__action-select" :options="addActionOptions()" :consistent-menu-width="false" size="small" placeholder="选择动作" aria-label="新增动作" />
            <n-button dashed size="small" :disabled="!newActionKind || !canAddAction(newActionKind)" @click="addActionRow">添加动作</n-button>
          </div>
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
.node-configuration-panel__action-rows { display: grid; gap: 9px; }
.node-configuration-panel__action-row { display: grid; gap: 7px; padding: 9px; border: 1px solid var(--flow-edge-color); border-radius: 4px; }
.node-configuration-panel__action-main { display: grid; grid-template-columns: minmax(132px, 1fr) auto; align-items: center; gap: 7px; }
.node-configuration-panel__action-select { min-width: 132px; }
.node-configuration-panel__row-count, .node-configuration-panel__row-tools { display: flex; align-items: center; gap: 5px; font-size: 12px; }
.node-configuration-panel__row-tools { grid-column: 1 / -1; justify-content: flex-end; }
.node-configuration-panel__row-count :deep(.n-input-number) { width: 76px; }
.node-configuration-panel__fixed-count { min-width: 76px; color: var(--flow-label-color); text-align: center; }
.node-configuration-panel__add-action { display: grid; grid-template-columns: minmax(132px, 1fr) auto; gap: 8px; margin-top: 14px; padding-top: 12px; border-top: 1px solid var(--flow-edge-color); }
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
