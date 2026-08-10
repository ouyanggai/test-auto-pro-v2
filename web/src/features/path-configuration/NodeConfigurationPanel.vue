<script setup lang="ts">
import {
  NAlert,
  NButton,
  NDatePicker,
  NEmpty,
  NInput,
  NInputNumber,
  NRadioButton,
  NRadioGroup,
  NSelect,
  NSwitch,
  NTag,
} from 'naive-ui'
import type { SelectOption } from 'naive-ui'

import { parsePathConfigValue } from './logic'
import type {
  PathConfigAction,
  PathConfigDraft,
  PathConfigField,
  PathConfigNode,
  PathConfigPerson,
} from './types'

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
  hasNextPath: boolean
}>()

const emit = defineEmits<{
  updateField: [field: PathConfigField, value: unknown]
  updateAction: [action: PathConfigAction, value: string]
  updatePerson: [person: PathConfigPerson, value: string[]]
  save: []
  backToPlan: []
  configureNext: []
}>()

// fieldValue 只解析当前节点的后端受约束值，控件不接触目标字段编码或原始 JSON。
function fieldValue(field: PathConfigField): unknown {
  return parsePathConfigValue(field, props.draft.fields[field.key] ?? '')
}

// dateFieldValue 给 Naive UI 严格日期控件提供格式化值，空值保持 null。
function dateFieldValue(field: PathConfigField): string | null {
  const value = fieldValue(field)
  return typeof value === 'string' && value.trim() !== '' ? value : null
}

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

// fieldOptions 转为 Naive UI 的只读选项结构，不附带任何目标原始字段信息。
function fieldOptions(field: PathConfigField): SelectOption[] {
  return field.options.map((option) => ({ label: option.label, value: option.value }))
}

// personOptions 转为 Naive UI 的不透明人员候选选项。
function personOptions(person: PathConfigPerson): SelectOption[] {
  return person.options.map((option) => ({ label: option.label, value: option.value }))
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

        <section v-if="node.fields.length" class="node-configuration-panel__section" aria-labelledby="node-fields-heading">
          <h3 id="node-fields-heading">表单数据</h3>
          <div
            v-for="field in node.fields"
            :key="field.key"
            class="node-configuration-panel__field"
            :class="{ 'node-configuration-panel__field--affected': field.affected }"
          >
            <label>
              <span>{{ field.name }}</span>
              <n-tag v-if="field.required" size="tiny" :bordered="false" type="error">必填</n-tag>
              <n-tag v-if="field.affected" size="tiny" :bordered="false" type="warning">需重新确认</n-tag>
            </label>
            <n-input
              v-if="field.type === 'text'"
              :value="String(fieldValue(field))"
              :disabled="node.lineBlocked || !field.editable"
              :placeholder="field.required ? '请输入必填值' : '选填'"
              @update:value="(value) => emit('updateField', field, value)"
            />
            <n-input-number
              v-else-if="field.type === 'number'"
              :value="typeof fieldValue(field) === 'number' ? fieldValue(field) as number : Number(fieldValue(field)) || null"
              :disabled="node.lineBlocked || !field.editable"
              :placeholder="field.required ? '请输入必填值' : '选填'"
              @update:value="(value) => emit('updateField', field, value)"
            />
            <n-date-picker
              v-else-if="field.type === 'date'"
              type="date"
              value-format="yyyy-MM-dd"
              :formatted-value="dateFieldValue(field)"
              :disabled="node.lineBlocked || !field.editable"
              placeholder="请选择日期"
              @update:formatted-value="(value) => emit('updateField', field, value)"
            />
            <n-date-picker
              v-else-if="field.type === 'dateTime'"
              type="datetime"
              value-format="yyyy-MM-dd HH:mm:ss"
              :formatted-value="dateFieldValue(field)"
              :disabled="node.lineBlocked || !field.editable"
              placeholder="请选择日期时间"
              @update:formatted-value="(value) => emit('updateField', field, value)"
            />
            <n-select
              v-else-if="field.type === 'singleSelect'"
              :value="String(fieldValue(field))"
              :options="fieldOptions(field)"
              :disabled="node.lineBlocked || !field.editable"
              :placeholder="field.required ? '请选择必填项' : '请选择'"
              @update:value="(value) => emit('updateField', field, value)"
            />
            <n-select
              v-else-if="field.type === 'multiSelect'"
              multiple
              :value="Array.isArray(fieldValue(field)) ? fieldValue(field) as string[] : []"
              :options="fieldOptions(field)"
              :disabled="node.lineBlocked || !field.editable"
              :placeholder="field.required ? '请选择必填项' : '请选择'"
              @update:value="(value) => emit('updateField', field, value)"
            />
            <n-switch
              v-else-if="field.type === 'switch'"
              :value="fieldValue(field) === true"
              :disabled="node.lineBlocked || !field.editable"
              @update:value="(value) => emit('updateField', field, value)"
            />
            <p v-if="field.note">{{ field.note }}</p>
          </div>
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
          v-if="!node.fields.length && !node.persons.length && !node.actions.length && !node.gaps.length && !node.requirements.length"
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
          路径节点配置已保存
        </n-alert>
        <span v-else-if="missingCount">还有 {{ missingCount }} 个必填项未完成</span>
        <span v-else>保存会校验整条路径的当前目标模板</span>
        <div class="node-configuration-panel__footer-actions">
          <template v-if="savedSuccessfully">
            <n-button size="small" @click="emit('backToPlan')">返回计划详情</n-button>
            <n-button v-if="hasNextPath" size="small" type="primary" @click="emit('configureNext')">配置下一条</n-button>
          </template>
          <n-button v-else type="primary" :loading="saving" :disabled="saveDisabled" @click="emit('save')">
            保存节点配置
          </n-button>
        </div>
      </footer>
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
.node-configuration-panel__field p,
.node-configuration-panel__person p {
  margin: 4px 0 0;
  color: var(--flow-label-color);
  font-size: 12px;
  line-height: 1.5;
  opacity: 0.76;
}

.node-configuration-panel__field,
.node-configuration-panel__person,
.node-configuration-panel__action {
  display: grid;
  gap: 6px;
  margin-bottom: 12px;
}

.node-configuration-panel__field label,
.node-configuration-panel__person label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
}

.node-configuration-panel__field--affected {
  padding: 8px;
  border: 1px solid var(--warning-color, var(--flow-direction-color));
  border-radius: 4px;
}

.node-configuration-panel__readonly {
  padding: 7px 9px;
  background: color-mix(in srgb, var(--flow-edge-color) 18%, transparent);
  border-radius: 4px;
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
