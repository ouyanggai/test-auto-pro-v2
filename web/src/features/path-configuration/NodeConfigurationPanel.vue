<script setup lang="ts">
import { NAlert, NButton, NEmpty, NSelect, NTag } from 'naive-ui'
import { computed } from 'vue'
import ActionOrchestrationEditor from './ActionOrchestrationEditor.vue'
import { containerActionsDraft, nodeActionContainer, normalizedPersonStrategy, pathConfigurationMessage, pathConfigurationStatusName, resolvedPersonStrategySelection, summarizePathConfigPersonItems } from './logic'
import type { PathActionContainer, PathConfigConfiguredActionInput, PathConfigDraft, PathConfigNode, PathConfigPerson, PathConfigPersonStrategyInput } from './types'

const props = defineProps<{ node: PathConfigNode | null; draft: PathConfigDraft; saving: boolean; readOnly: boolean; saveDisabled: boolean; saveAllDisabled: boolean; missingCount: number; saveError: string; saveDetails: Array<{ kind: string; name: string; reason: string }>; savedSuccessfully: boolean; formComplete: boolean; instanceContainer?: PathActionContainer | null; instanceSavedActions?: PathConfigConfiguredActionInput[] }>()
const emit = defineEmits<{ updatePersonStrategy: [person: PathConfigPerson, value: PathConfigPersonStrategyInput]; updateActionConfiguration: [nodeKey: string, value: PathConfigConfiguredActionInput[]]; save: []; saveAll: []; backToPlan: []; openForm: [] }>()

const container = computed(() => props.node ? nodeActionContainer(props.node) : null)
// savedActions 只保留当前节点已确认的独立动作记录。
const savedActions = computed(() => container.value ? containerActionsDraft(container.value, props.draft) : [])

// personDraft 返回当前人员策略草稿。
function personDraft(person: PathConfigPerson) { return normalizedPersonStrategy(person, props.draft.personStrategies[person.key]) }
// personOptions 生成不透明人员候选。
function personOptions(person: PathConfigPerson) { return person.options.map(option => ({ label: option.label, value: option.value })) }
// strategyOptions 生成当前模板允许的策略候选。
function strategyOptions(person: PathConfigPerson) { return person.strategies.map(option => ({ label: option.label, value: option.value })) }
// updatePersonStrategy 更新当前节点处理人员草稿。
function updatePersonStrategy(person: PathConfigPerson, patch: Partial<PathConfigPersonStrategyInput>) { const next = { ...personDraft(person), ...patch, key: person.key }; next.selected = resolvedPersonStrategySelection(person, next); emit('updatePersonStrategy', person, next) }
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
        <ActionOrchestrationEditor
          v-if="container"
          :container="container"
          title="节点动作"
          :saved-actions="savedActions"
          :read-only="readOnly"
          :blocked="node.lineBlocked"
          :person-strategies="draft.personStrategies"
          :instance-container="instanceContainer"
          :instance-saved-actions="instanceSavedActions"
          @update="(key, value) => emit('updateActionConfiguration', key, value)"
        />
      </section>
    </div>

    <footer class="node-configuration-panel__footer">
      <div class="save-status">
        <n-alert v-if="readOnly" type="info" :show-icon="false">当前计划只能查看</n-alert>
        <n-alert v-if="saveError" type="error" :show-icon="false">{{ pathConfigurationMessage(saveError) }}</n-alert>
        <ul v-if="saveDetails.length" class="save-details">
          <li v-for="detail in saveDetails" :key="`${detail.kind}-${detail.name}`">{{ detail.name }}：{{ pathConfigurationMessage(detail.reason) }}</li>
        </ul>
        <span v-if="!saveError && missingCount">还有 {{ missingCount }} 项未满足配置要求</span>
      </div>
      <div v-if="!readOnly" class="save-actions">
        <n-button secondary :loading="saving" :disabled="saveAllDisabled" @click="emit('saveAll')">保存全部节点</n-button>
        <n-button class="save-button" type="primary" :loading="saving" :disabled="saveDisabled" @click="emit('save')">保存当前节点</n-button>
      </div>
    </footer>

    <n-empty v-if="!node.persons.length && !node.actionConfiguration.catalog.length" description="此节点没有需要配置的内容" />
  </section>
</template>

<style scoped>
.node-configuration-panel{height:100%;display:flex;flex-direction:column;gap:12px;padding:16px}
.node-configuration-panel__header{display:flex;align-items:center;justify-content:space-between;gap:10px}
.node-configuration-panel__body{flex:1;min-height:0;overflow:auto;display:flex;flex-direction:column;gap:16px}
.node-configuration-panel__section{border-top:1px solid #e5e7eb;padding-top:14px}
.person-row{display:flex;flex-direction:column;align-items:stretch;gap:7px;margin-top:12px}
.person-controls{display:flex;flex-direction:column;gap:7px}
.node-configuration-panel p,.node-configuration-panel small{color:#64748b}
.node-configuration-panel__footer{display:flex;flex-direction:column;align-items:stretch;gap:10px;margin:0 -16px -16px;padding:14px 16px 16px;border-top:1px solid #e5e7eb;background:#fafafa}
.save-status{min-height:20px;color:#64748b}
.save-details{margin:6px 0 0;padding-left:18px}
.save-button{align-self:flex-end;min-width:132px}
.node-configuration-panel h2,.node-configuration-panel h3{margin:0}
.node-configuration-panel p{margin:0}
.save-actions{display:flex;justify-content:flex-end;gap:8px}
@media (max-width:680px){.node-configuration-panel{padding:12px}.save-button{width:100%}.node-configuration-panel__footer{margin:0 -12px -12px;padding:12px}.save-actions{flex-direction:column-reverse}.save-actions :deep(.n-button){width:100%}}
</style>
