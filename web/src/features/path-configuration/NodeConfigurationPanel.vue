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

import { copyPathConfigActionPlan, normalizedPersonStrategy, pathConfigActionPlanInput, resolvedPersonStrategySelection, summarizePathConfigPersonItems } from './logic'
import type {
  PathConfigActionCatalogItem,
  PathConfigActionKind,
  PathConfigActionPlanInput,
  PathConfigDraft,
  PathConfigNode,
  PathConfigPerson,
  PathConfigPersonStrategyInput,
} from './types'

const PERSON_PREVIEW_LIMIT = 3
const MAX_SAFE_PERSON_SEED = Number.MAX_SAFE_INTEGER
const personDetailsOpen = ref(false)
const detailedPerson = ref<PathConfigPerson | null>(null)

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
}>()

const emit = defineEmits<{
  updatePersonStrategy: [person: PathConfigPerson, value: PathConfigPersonStrategyInput]
  updateActionPlan: [nodeKey: string, value: PathConfigActionPlanInput]
  save: []
  backToPlan: []
  openForm: []
}>()

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

// addSignNode 新增一个拥有独立人员策略的审批节点；多个条目代表多个真实加签节点。
function addSignNode() {
  const definition = actionDefinition('add_sign')
  if (!definition?.enabled || !definition.person || !actionPlan.value || actionPlan.value.addSignNodes.length >= 10) return
  const next = copyPathConfigActionPlan(actionPlan.value)
  next.addSignNodes.push({ person: personDraft(definition.person) })
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
  next.addSignNodes[index] = { person: updated }
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
          <n-tag size="small" :bordered="false" :type="node.status === 'configured' ? 'success' : node.status === 'affected' ? 'error' : node.status === 'runtime' ? 'info' : 'warning'">
            {{ node.statusName }}
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
            <p v-if="person.note">{{ person.note }}</p>
          </div>
        </section>

        <section v-if="node.actionPlan.catalog.length" class="node-configuration-panel__section" aria-labelledby="node-actions-heading">
          <div class="node-configuration-panel__section-heading">
            <h3 id="node-actions-heading">动作计划</h3>
            <n-popover trigger="click" placement="bottom-end" :width="340" scrollable>
              <template #trigger><n-button class="node-configuration-panel__action-info" circle size="small" aria-label="查看动作规则" title="查看动作规则"><n-icon><InformationCircleOutline /></n-icon></n-button></template>
              <div class="node-configuration-panel__action-rules">
                <strong>动作说明</strong>
                <p class="node-configuration-panel__runtime-note">流程后续再次进入节点时，会重新读取真实实例、待办、权限、分支和人员。当前配置不模拟网络重试或流程循环。</p>
                <n-scrollbar style="max-height: 320px">
                  <ul>
                    <li v-for="item in node.actionPlan.catalog" :key="item.kind">
                      <div><strong>{{ item.label }}</strong><n-tag size="tiny" :bordered="false" :type="actionRuleReason(item) ? 'default' : 'success'">{{ actionRuleStatus(item) }}</n-tag></div>
                      <p>{{ item.description }}</p>
                      <small v-if="actionRuleReason(item)">{{ actionRuleReason(item) }}</small>
                    </li>
                  </ul>
                </n-scrollbar>
              </div>
            </n-popover>
          </div>
          <n-alert v-if="node.actionPlan.note" type="warning" :show-icon="false" size="small">{{ node.actionPlan.note }}</n-alert>
          <div v-if="actionPlan" class="node-configuration-panel__action-plan">
            <!-- 发起节点只有固定提交动作。 -->
            <div v-if="node.kind === 'start'" class="node-configuration-panel__action-row">
              <div class="node-configuration-panel__action-row-head">
                <strong>提交</strong>
                <n-tag size="tiny" :bordered="false" type="success">发起动作</n-tag>
              </div>
              <p class="node-configuration-panel__runtime-note">提交当前发起节点并进入后续流程；执行时仍会核对模板、表单和账号。</p>
            </div>

            <!-- 审批/协同节点：加签与处理结果统一为动作列表，每个动作一行。 -->
            <template v-else>
              <div v-for="(addSign, index) in actionPlan.addSignNodes" :key="`add-sign-${index}`" class="node-configuration-panel__action-row">
                <div class="node-configuration-panel__action-row-head">
                  <strong>加签节点 {{ index + 1 }}</strong>
                  <div class="node-configuration-panel__action-row-tools">
                    <n-popover v-if="actionPerson('add_sign')" trigger="click" placement="bottom-start" :width="320">
                      <template #trigger>
                        <n-button size="tiny" secondary>选择人员</n-button>
                      </template>
                      <div class="node-configuration-panel__person-picker">
                        <span class="node-configuration-panel__parameter-title">加签节点处理人</span>
                        <n-select :value="addSign.person.strategy" :options="strategyOptions(requiredActionPerson('add_sign'))" @update:value="value => updateAddSignPerson(index, requiredActionPerson('add_sign'), { strategy: value })" />
                        <n-select
                          v-if="addSign.person.strategy === 'manual'"
                          :multiple="requiredActionPerson('add_sign').multiple"
                          filterable
                          :value="requiredActionPerson('add_sign').multiple ? addSign.person.selected : (addSign.person.selected[0] ?? null)"
                          :options="personOptions(requiredActionPerson('add_sign'))"
                          @update:value="value => updateAddSignPerson(index, requiredActionPerson('add_sign'), { selected: Array.isArray(value) ? value : (value ? [value] : []) })"
                        />
                        <n-input-number v-if="addSign.person.strategy === 'random'" :value="addSign.person.seed" :min="1" :max="MAX_SAFE_PERSON_SEED" aria-label="加签节点人员随机种子" @update:value="value => updateAddSignPerson(index, requiredActionPerson('add_sign'), { seed: value || 1 })" />
                      </div>
                    </n-popover>
                    <n-button quaternary circle size="tiny" title="上移" aria-label="上移加签节点" :disabled="index === 0" @click="moveAddSignNode(index, -1)"><n-icon><ArrowUpOutline /></n-icon></n-button>
                    <n-button quaternary circle size="tiny" title="下移" aria-label="下移加签节点" :disabled="index === actionPlan.addSignNodes.length - 1" @click="moveAddSignNode(index, 1)"><n-icon><ArrowDownOutline /></n-icon></n-button>
                    <n-button quaternary circle size="tiny" title="删除" aria-label="删除加签节点" @click="removeAddSignNode(index)"><n-icon><CloseOutline /></n-icon></n-button>
                  </div>
                </div>
                <div class="node-configuration-panel__action-row-selected">
                  <template v-if="actionPerson('add_sign') && selectedPersonNames(requiredActionPerson('add_sign'), addSign.person).length">
                    <n-tag v-for="name in selectedPersonNames(requiredActionPerson('add_sign'), addSign.person).slice(0, PERSON_PREVIEW_LIMIT)" :key="name" size="small" :bordered="false" type="success">{{ name }}</n-tag>
                    <span v-if="selectedPersonNames(requiredActionPerson('add_sign'), addSign.person).length > PERSON_PREVIEW_LIMIT" class="node-configuration-panel__more">等 {{ selectedPersonNames(requiredActionPerson('add_sign'), addSign.person).length }} 人</span>
                  </template>
                  <span v-else class="node-configuration-panel__row-empty">尚未选择处理人</span>
                </div>
              </div>

              <!-- 处理结果行 -->
              <div class="node-configuration-panel__action-row node-configuration-panel__action-row--result">
                <div class="node-configuration-panel__action-row-head">
                  <strong>处理结果</strong>
                  <n-select size="small" class="node-configuration-panel__result-select" :value="resultKind" :options="resultOptions()" :consistent-menu-width="false" aria-label="处理结果" @update:value="updateResult" />
                </div>
                <p v-if="actionDefinition(resultKind)?.requiresTarget && node.actionPlan.rollbackTargets.length === 1" class="node-configuration-panel__readonly">回退至：{{ node.actionPlan.rollbackTargets[0].label }}</p>
                <template v-if="actionDefinition(resultKind)?.requiresPerson && actionPerson(resultKind)">
                  <div class="node-configuration-panel__action-row-selected">
                    <n-popover trigger="click" placement="bottom-start" :width="320">
                      <template #trigger>
                        <n-button size="tiny" secondary>选择移交人员</n-button>
                      </template>
                      <div class="node-configuration-panel__person-picker">
                        <span class="node-configuration-panel__parameter-title">移交人员</span>
                        <n-select :value="resultPerson()?.strategy ?? requiredActionPerson(resultKind).strategy" :options="strategyOptions(requiredActionPerson(resultKind))" @update:value="value => updateResultPerson(requiredActionPerson(resultKind), { strategy: value })" />
                        <n-select
                          v-if="(resultPerson()?.strategy ?? requiredActionPerson(resultKind).strategy) === 'manual'"
                          :multiple="requiredActionPerson(resultKind).multiple"
                          filterable
                          :value="requiredActionPerson(resultKind).multiple ? (resultPerson()?.selected ?? []) : (resultPerson()?.selected?.[0] ?? null)"
                          :options="personOptions(requiredActionPerson(resultKind))"
                          @update:value="value => updateResultPerson(requiredActionPerson(resultKind), { selected: Array.isArray(value) ? value : (value ? [value] : []) })"
                        />
                        <n-input-number v-if="(resultPerson()?.strategy ?? requiredActionPerson(resultKind).strategy) === 'random'" :value="resultPerson()?.seed ?? requiredActionPerson(resultKind).strategySeed" :min="1" :max="MAX_SAFE_PERSON_SEED" aria-label="移交人员随机种子" @update:value="value => updateResultPerson(requiredActionPerson(resultKind), { seed: value || 1 })" />
                      </div>
                    </n-popover>
                    <n-tag v-for="name in selectedResultPersonNames(resultKind).slice(0, PERSON_PREVIEW_LIMIT)" :key="name" size="small" :bordered="false" type="success">{{ name }}</n-tag>
                  </div>
                </template>
              </div>

              <!-- 添加动作：只追加新的加签动作，处理结果保持唯一。 -->
              <div class="node-configuration-panel__action-add">
                <n-button v-if="actionDefinition('add_sign')?.enabled" dashed size="small" :disabled="actionPlan.addSignNodes.length >= 10" @click="addSignNode">
                  <template #icon><n-icon><AddOutline /></n-icon></template>添加动作
                </n-button>
                <span v-else-if="actionDefinition('add_sign')" class="node-configuration-panel__runtime-note">{{ addSignDisabledReason() }}</span>
              </div>
            </template>
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
