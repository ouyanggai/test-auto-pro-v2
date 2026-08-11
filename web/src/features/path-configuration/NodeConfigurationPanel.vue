<script setup lang="ts">
import {
  NAlert,
  NButton,
  NCard,
  NEmpty,
  NModal,
  NRadioButton,
  NRadioGroup,
  NScrollbar,
  NSelect,
  NTag,
} from 'naive-ui'
import type { SelectOption } from 'naive-ui'
import { ref } from 'vue'

import { summarizePathConfigPersonItems } from './logic'
import type {
  PathConfigAction,
  PathConfigDraft,
  PathConfigNode,
  PathConfigPerson,
} from './types'

const PERSON_PREVIEW_LIMIT = 3
const personDetailsOpen = ref(false)
const detailedPerson = ref<PathConfigPerson | null>(null)

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
  updateAction: [action: PathConfigAction, value: string]
  updatePerson: [person: PathConfigPerson, value: string[]]
  save: []
  backToPlan: []
  openForm: []
}>()

// actionValue 返回节点动作草稿；缺失时使用目标模板给出的默认动作。
function actionValue(action: PathConfigAction): string {
  return props.draft.actions[action.key] ?? action.default
}

// personValue 返回人员选择草稿副本，避免直接修改父组件持有的数组。
function personValue(person: PathConfigPerson): string[] {
  return [...(props.draft.persons[person.key] ?? person.selected)]
}

// updateSinglePerson 把单选控件值统一收敛为后端人员数组语义。
function updateSinglePerson(person: PathConfigPerson, value: string | null) {
  emit('updatePerson', person, value ? [value] : [])
}

// personOptions 转为 Naive UI 的不透明人员候选选项。
function personOptions(person: PathConfigPerson): SelectOption[] {
  return person.options.map((option) => ({ label: option.label, value: option.value }))
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
</script>

<template>
  <section class="node-configuration-panel">
    <template v-if="node">
      <header class="node-configuration-panel__header">
        <div>
          <span class="node-configuration-panel__eyebrow">当前节点</span>
          <h2>{{ node.name }}</h2>
        </div>
        <div class="node-configuration-panel__tags">
          <n-tag size="small" :bordered="false">{{ node.typeName }}</n-tag>
          <n-tag
            size="small"
            :bordered="false"
            :type="node.status === 'configured' ? 'success' : node.status === 'affected' ? 'error' : node.status === 'runtime' ? 'info' : 'warning'"
          >
            {{ node.statusName }}
          </n-tag>
        </div>
      </header>

      <div class="node-configuration-panel__scroll">
        <n-alert v-for="(warning, index) in warnings" :key="`warning-${index}`" type="warning" :show-icon="false" size="small">
          {{ warning }}
        </n-alert>
        <n-alert v-if="node.lineBlocked" type="warning" :show-icon="false" size="small">
          前序审批选择了不同意，本节点及后续不再按原路径继续。
        </n-alert>

        <section v-if="node.requirements.length" class="node-configuration-panel__section" aria-labelledby="node-requirements-heading">
          <h3 id="node-requirements-heading">模板要求</h3>
          <ul class="node-configuration-panel__requirements">
            <li v-for="(requirement, index) in node.requirements" :key="index">
              <strong>{{ requirement.title }}</strong>
              <span>{{ requirement.detail }}</span>
            </li>
          </ul>
        </section>

        <section v-if="node.persons.length" class="node-configuration-panel__section" aria-labelledby="node-persons-heading">
          <h3 id="node-persons-heading">处理人员</h3>
          <div v-for="(person, index) in node.persons" :key="`person-${index}`" class="node-configuration-panel__person">
            <label>
              <span>{{ person.title }}</span>
              <n-tag v-if="person.required" size="tiny" :bordered="false" type="error">必选</n-tag>
              <n-tag v-if="person.mode === 'runtime'" size="tiny" :bordered="false" type="info">运行时确定</n-tag>
            </label>
            <n-select
              v-if="person.editable && person.multiple"
              multiple
              :value="personValue(person)"
              :options="personOptions(person)"
              :disabled="node.lineBlocked"
              :placeholder="person.minCount > 1 ? `至少选择 ${person.minCount} 人` : '请选择处理人'"
              @update:value="(value) => emit('updatePerson', person, value)"
            />
            <n-select
              v-else-if="person.editable"
              :value="personValue(person)[0] ?? null"
              :options="personOptions(person)"
              :disabled="node.lineBlocked"
              placeholder="请选择处理人"
              @update:value="(value) => updateSinglePerson(person, value)"
            />
            <p v-else class="node-configuration-panel__readonly">{{ person.detail }}</p>
            <ul v-if="person.items.length" class="node-configuration-panel__person-items" aria-label="目标模板已配置对象">
              <li v-for="(item, itemIndex) in personItemSummary(person).preview" :key="`${item.category}-${item.name}-${itemIndex}`">
                <n-tag size="tiny" :bordered="false">{{ item.category }}</n-tag>
                <span>{{ item.name }}</span>
                <small v-if="item.count > 1">{{ item.count }} 项</small>
              </li>
            </ul>
            <n-button
              v-if="personItemSummary(person).total > PERSON_PREVIEW_LIMIT"
              text
              type="primary"
              size="small"
              class="node-configuration-panel__person-more"
              @click="openPersonDetails(person)"
            >
              查看全部 {{ personItemSummary(person).total }} 项
            </n-button>
            <p v-if="person.note">{{ person.note }}</p>
          </div>
        </section>

        <section v-if="node.actions.length" class="node-configuration-panel__section" aria-labelledby="node-actions-heading">
          <h3 id="node-actions-heading">节点动作</h3>
          <div v-for="action in node.actions" :key="action.key" class="node-configuration-panel__action">
            <span>{{ action.label }}</span>
            <n-radio-group
              v-if="action.kind === 'agree_disagree'"
              :value="actionValue(action)"
              :disabled="node.lineBlocked"
              @update:value="(value) => emit('updateAction', action, value as string)"
            >
              <n-radio-button v-for="option in action.options" :key="option.value" :value="option.value">
                {{ option.label }}
              </n-radio-button>
            </n-radio-group>
            <n-tag v-else :bordered="false" type="info">固定提交</n-tag>
            <n-alert
              v-if="action.kind === 'agree_disagree' && actionValue(action) === 'disagree'"
              type="warning"
              :show-icon="false"
              size="small"
            >
              {{ action.disagreeWarning }}
            </n-alert>
          </div>
        </section>

        <section v-if="node.gaps.length" class="node-configuration-panel__section" aria-labelledby="node-gaps-heading">
          <h3 id="node-gaps-heading">暂不支持</h3>
          <n-alert v-for="(gap, index) in node.gaps" :key="index" type="warning" :show-icon="false" size="small">
            <strong>{{ gap.name }}</strong>：{{ gap.reason }}
          </n-alert>
        </section>

        <n-empty
          v-if="!node.persons.length && !node.actions.length && !node.gaps.length && !node.requirements.length"
          size="small"
          description="此节点没有需要配置的内容"
        />
      </div>

      <footer class="node-configuration-panel__footer">
        <n-alert v-if="saveError" type="error" :show-icon="false" size="small">
          {{ saveError }}
          <ul v-if="saveDetails.length">
            <li v-for="(item, index) in saveDetails" :key="index">{{ item.name }}：{{ item.reason }}</li>
          </ul>
        </n-alert>
        <n-alert v-else-if="savedSuccessfully" type="success" :show-icon="false" size="small">
          {{ formComplete ? '当前路径的节点与表单配置均已完成' : '当前节点配置已保存，节点配置已完成' }}
        </n-alert>
        <span v-else-if="missingCount">还有 {{ missingCount }} 项未满足模板要求</span>
        <span v-else>保存只更新当前节点的人员与动作</span>
        <div class="node-configuration-panel__footer-actions">
          <template v-if="savedSuccessfully">
            <n-button size="small" @click="emit('backToPlan')">返回计划详情</n-button>
            <n-button v-if="!formComplete" size="small" type="primary" @click="emit('openForm')">配置表单数据</n-button>
          </template>
          <n-button v-else type="primary" :loading="saving" :disabled="saveDisabled" @click="emit('save')">
            保存当前节点
          </n-button>
        </div>
      </footer>

      <n-modal :show="personDetailsOpen" :mask-closable="true" @update:show="(show) => { if (!show) closePersonDetails() }">
        <n-card v-if="detailedPerson" class="node-configuration-panel__person-modal" :bordered="false" role="dialog" aria-modal="true" aria-labelledby="person-details-title">
          <template #header>
            <div class="node-configuration-panel__person-modal-heading">
              <span>目标模板已配置对象</span>
              <h3 id="person-details-title">{{ detailedPerson.title }} · {{ personItemSummary(detailedPerson).total }} 项</h3>
            </div>
          </template>
          <template #header-extra>
            <n-button quaternary size="small" aria-label="关闭人员详情" @click="closePersonDetails">关闭</n-button>
          </template>
          <n-scrollbar class="node-configuration-panel__person-modal-scroll" style="max-height: 360px">
            <ul>
              <li v-for="(item, itemIndex) in detailedPerson.items" :key="`${item.category}-${item.name}-${itemIndex}`">
                <n-tag size="small" :bordered="false">{{ item.category }}</n-tag>
                <span>{{ item.name }}</span>
                <small v-if="item.count > 1">对应 {{ item.count }} 项目标范围</small>
              </li>
            </ul>
          </n-scrollbar>
        </n-card>
      </n-modal>
    </template>
    <n-empty v-else class="node-configuration-panel__empty" description="请在当前路径上选择一个节点" />
  </section>
</template>

<style scoped>
.node-configuration-panel {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto;
  width: 100%;
  height: 100%;
  color: var(--flow-label-color);
  background: var(--flow-surface-color);
}

.node-configuration-panel__header,
.node-configuration-panel__footer {
  padding: 12px 14px;
  background: var(--flow-surface-color);
}

.node-configuration-panel__header {
  border-bottom: 1px solid var(--flow-edge-color);
}

.node-configuration-panel__header h2 {
  margin: 3px 0 8px;
  font-size: 17px;
  line-height: 1.35;
}

.node-configuration-panel__eyebrow {
  color: var(--flow-label-color);
  font-size: 12px;
  opacity: 0.7;
}

.node-configuration-panel__tags,
.node-configuration-panel__footer-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.node-configuration-panel__scroll {
  min-height: 0;
  padding: 12px 14px;
  overflow-y: auto;
  overscroll-behavior: contain;
  scrollbar-gutter: stable;
}

.node-configuration-panel__section + .node-configuration-panel__section {
  margin-top: 18px;
}

.node-configuration-panel__section h3 {
  margin: 0 0 9px;
  font-size: 14px;
}

.node-configuration-panel__requirements {
  display: grid;
  gap: 7px;
  padding: 0;
  margin: 0;
  list-style: none;
}

.node-configuration-panel__requirements li {
  display: grid;
  gap: 2px;
  padding-left: 9px;
  border-left: 2px solid var(--flow-edge-color);
}

.node-configuration-panel__requirements span,
.node-configuration-panel__readonly,
.node-configuration-panel__person p {
  margin: 4px 0 0;
  color: var(--flow-label-color);
  font-size: 12px;
  line-height: 1.5;
  opacity: 0.76;
}

.node-configuration-panel__person,
.node-configuration-panel__action {
  display: grid;
  gap: 6px;
  margin-bottom: 12px;
}

.node-configuration-panel__person label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
}

.node-configuration-panel__readonly {
  padding: 7px 9px;
  background: color-mix(in srgb, var(--flow-edge-color) 18%, transparent);
  border-radius: 4px;
}

.node-configuration-panel__person-items,
.node-configuration-panel__person-modal ul {
  display: grid;
  gap: 7px;
  padding: 0;
  margin: 0;
  list-style: none;
}

.node-configuration-panel__person-items li,
.node-configuration-panel__person-modal li {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 7px;
  min-width: 0;
  font-size: 12px;
}

.node-configuration-panel__person-items li > span,
.node-configuration-panel__person-modal li > span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.node-configuration-panel__person-items small,
.node-configuration-panel__person-modal small {
  color: var(--flow-label-color);
  opacity: 0.65;
}

.node-configuration-panel__person-more {
  justify-self: start;
}

.node-configuration-panel__person-modal {
  width: min(520px, calc(100vw - 32px));
  max-height: min(520px, calc(100dvh - 48px));
}

.node-configuration-panel__person-modal h3 {
  margin: 3px 0 0;
  font-size: 16px;
}

.node-configuration-panel__person-modal-heading > span {
  font-size: 12px;
  opacity: 0.7;
}

.node-configuration-panel__person-modal-scroll {
  min-height: 0;
  padding: 12px 4px 4px 0;
}

.node-configuration-panel__footer {
  display: grid;
  gap: 10px;
  border-top: 1px solid var(--flow-edge-color);
  font-size: 12px;
}

.node-configuration-panel__footer-actions {
  justify-content: flex-end;
}

.node-configuration-panel__footer ul {
  margin: 5px 0 0;
  padding-left: 18px;
}

.node-configuration-panel__empty {
  align-self: center;
  padding: 24px;
}
</style>
