<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import {
  NAlert, NButton, NCard, NDataTable, NIcon, NInput, NSelect, NTag,
  NPopconfirm, NSpin, useMessage, type DataTableColumns, type SelectOption,
} from 'naive-ui'
import { useRouter } from 'vue-router'
import { ArrowBackOutline } from '@vicons/ionicons5'

import {
  addAddSignExclusion,
  fetchAddSignExclusions,
  removeAddSignExclusion,
  searchAddSignCandidates,
  type AddSignCandidate,
  type AddSignExclusionItem,
} from '../features/add-sign-exclusion/api'

const message = useMessage()
const router = useRouter()
const items = ref<AddSignExclusionItem[]>([])
const loading = ref(true)
const submitting = ref(false)
const errorMessage = ref('')
const nameFilter = ref('')

// 检索选人：输入姓名远程检索目标人员目录，点选后连同 personId 一并加入排除名单。
const searchLoading = ref(false)
const selectedPersonId = ref<string | null>(null)
const candidates = ref<AddSignCandidate[]>([])
const reason = ref('')
let searchToken = 0
let searchTimer: number | undefined

const total = computed(() => items.value.length)
const manualTotal = ref(0)
const autoTotal = ref(0)

// 候选转下拉选项：停用账号追加「已停用」标记，突出这类「加签后必停机」的人员，引导优先排除。
const candidateOptions = computed<SelectOption[]>(() => candidates.value.map((c) => ({
  label: c.enabled ? `${c.personName}${c.phone ? '（' + c.phone + '）' : ''}` : `${c.personName}${c.phone ? '（' + c.phone + '）' : ''} · 已停用`,
  value: c.personId,
})))

// handleSearch 远程检索，输入停顿 300ms 后触发，避免每个字符都打接口。
function handleSearch(query: string) {
  window.clearTimeout(searchTimer)
  const keyword = query.trim()
  if (!keyword) {
    candidates.value = []
    return
  }
  searchTimer = window.setTimeout(async () => {
    const token = ++searchToken
    searchLoading.value = true
    try {
      const result = await searchAddSignCandidates(keyword)
      if (token === searchToken) candidates.value = result.items
    }
    catch (error) {
      if (token === searchToken) message.error(error instanceof Error ? error.message : '检索目标人员失败')
    }
    finally {
      if (token === searchToken) searchLoading.value = false
    }
  }, 300)
}

// loadItems 读取排除名单，按当前姓名过滤条件查询。
async function loadItems() {
  loading.value = true
  errorMessage.value = ''
  try {
    const result = await fetchAddSignExclusions(nameFilter.value)
    items.value = result.items
    manualTotal.value = result.manualTotal ?? 0
    autoTotal.value = result.autoTotal ?? 0
  }
  catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '排除名单读取失败'
  }
  finally {
    loading.value = false
  }
}

// submit 把选中的候选人加入排除名单，personId/姓名/手机号都来自目标人员目录，无需手填。
async function submit() {
  const picked = candidates.value.find((c) => c.personId === selectedPersonId.value)
  if (!picked) {
    message.warning('请先检索并选择一位人员')
    return
  }
  submitting.value = true
  try {
    await addAddSignExclusion({ personId: picked.personId, personName: picked.personName, personPhone: picked.phone, reason: reason.value.trim() })
    message.success(`已把 ${picked.personName} 加入排除名单`)
    selectedPersonId.value = null
    reason.value = ''
    candidates.value = []
    await loadItems()
  }
  catch (error) {
    message.error(error instanceof Error ? error.message : '加入排除名单失败')
  }
  finally {
    submitting.value = false
  }
}

// remove 删除一条排除记录，成功后刷新列表。
async function remove(item: AddSignExclusionItem) {
  try {
    await removeAddSignExclusion(item.personId)
    message.success(`已把 ${item.personName} 移出排除名单`)
    await loadItems()
  }
  catch (error) {
    message.error(error instanceof Error ? error.message : '移出排除名单失败')
  }
}

const columns = computed<DataTableColumns<AddSignExclusionItem>>(() => [
  { title: '姓名', key: 'personName', width: 120 },
  { title: '手机号', key: 'personPhone', width: 140, render: (row) => row.personPhone || '—' },
  {
    title: '来源',
    key: 'source',
    width: 110,
    render: (row) => row.source === 'auto'
      ? h(NTag, { size: 'small', type: 'warning', bordered: false }, () => '系统自动')
      : h(NTag, { size: 'small', type: 'info', bordered: false }, () => '手动'),
  },
  { title: '排除原因', key: 'reason', render: (row) => row.reason || '—' },
  {
    title: '操作',
    key: 'actions',
    width: 100,
    // 系统自动排除（停用/无平台权限）由目标数据决定，不可手动移除；只有手动项可移除。
    render: (row) => row.source === 'auto'
      ? h('span', { style: 'color: var(--text-color-3)' }, '—')
      : h(
        NPopconfirm,
        { onPositiveClick: () => remove(row) },
        { trigger: () => h(NButton, { size: 'small', tertiary: true, type: 'error' }, () => '移除'), default: () => `确认把 ${row.personName} 移出排除名单？` },
      ),
  },
])

onMounted(loadItems)
</script>

<template>
  <section class="exclusion">
    <header class="exclusion__header">
      <div class="back-nav">
        <n-button
          quaternary
          size="small"
          class="back-btn"
          @click="router.push('/settings')"
        >
          <template #icon>
            <n-icon :component="ArrowBackOutline" />
          </template>
          返回系统设置
        </n-button>
      </div>
      <h1>人员排除</h1>
      <p>排除的人员不会被节点处理人、加签、移交、转发的随机选择选中，避免运行到其节点因无法登录而整条路径失败。账号已停用、或未分配当前平台权限的人员由系统自动排除（无需手动维护）；其余可手动添加。</p>
    </header>

    <n-alert v-if="errorMessage" type="error" :show-icon="false">{{ errorMessage }}</n-alert>

    <n-card title="新增排除人员" size="small">
      <div class="exclusion__add">
        <n-select
          v-model:value="selectedPersonId"
          filterable
          remote
          clearable
          placeholder="输入姓名检索目标人员，如 陈涛"
          :options="candidateOptions"
          :loading="searchLoading"
          :clear-filter-after-select="false"
          @search="handleSearch"
        />
        <n-input v-model:value="reason" placeholder="排除原因（可选），如：账号已停用无法登录" />
        <n-button type="primary" :loading="submitting" :disabled="!selectedPersonId" @click="submit">加入排除名单</n-button>
      </div>
      <p class="exclusion__hint">按姓名检索目标平台人员目录，点选后自动带上人员标识。标注「已停用」的账号无法登录，是加签后运行停机的主因，建议优先排除。</p>
    </n-card>

    <n-card size="small">
      <template #header>
        <div class="exclusion__list-header">
          <span>共 {{ total }} 人（手动 {{ manualTotal }} · 系统自动 {{ autoTotal }}）</span>
          <div class="exclusion__search">
            <n-input v-model:value="nameFilter" placeholder="按姓名搜索" clearable @keyup.enter="loadItems" />
            <n-button @click="loadItems">搜索</n-button>
          </div>
        </div>
      </template>
      <n-spin :show="loading">
        <n-data-table :columns="columns" :data="items" :bordered="false" size="small" />
      </n-spin>
    </n-card>
  </section>
</template>

<style scoped>
.exclusion { display: grid; gap: 16px; padding: 24px; }
.back-nav { margin-bottom: 8px; }
.back-btn { padding-left: 0; color: var(--text-color-2); font-weight: 500; transition: all 0.2s ease; }
.back-btn:hover { color: var(--primary-color); transform: translateX(-2px); }
.exclusion__header h1 { margin: 6px 0 4px; }
.exclusion__header p { margin: 0; color: var(--text-color-3); max-width: 720px; }
.exclusion__add { display: grid; grid-template-columns: minmax(0, 2fr) minmax(0, 2fr) auto; gap: 12px; align-items: center; }
.exclusion__hint { margin: 12px 0 0; color: var(--text-color-3); font-size: 13px; }
.exclusion__list-header { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.exclusion__search { display: flex; gap: 8px; }
@media (max-width: 760px) { .exclusion__add { grid-template-columns: 1fr; } }
</style>
