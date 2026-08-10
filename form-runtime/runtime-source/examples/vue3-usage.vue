<!--
  Vue 3 宿主应用集成示例

  展示如何在 Vue 3 项目中使用全屏 iframe 弹窗来调用 rsh-flow-components 的流程功能。
  这是 PortalView.vue 流程审批中心的简化版本。
-->
<template>
  <div class="flow-demo">
    <h2>流程审批中心</h2>

    <!-- Tab 栏 -->
    <div class="tabs">
      <button
        v-for="tab in tabs"
        :key="tab.key"
        :class="{ active: activeTab === tab.key }"
        @click="switchTab(tab.key)"
      >
        {{ tab.label }} ({{ tabCounts[tab.key] }})
      </button>
    </div>

    <!-- 列表 -->
    <div v-if="loading" class="loading">加载中...</div>
    <div v-else-if="items.length === 0" class="empty">暂无数据</div>
    <div v-else class="list">
      <div v-for="item in items" :key="item.id" class="list-item">
        <span class="title">{{ item.title }}</span>
        <span class="status">{{ item.flowStatusLabel }}</span>
        <div class="actions">
          <button
            v-for="action in getActions(activeTab, item)"
            :key="action.action"
            @click="handleAction(action.action, item)"
          >
            {{ action.label }}
          </button>
        </div>
      </div>
    </div>

    <!-- 全屏 iframe 弹窗 -->
    <Teleport to="body">
      <div v-if="iframeVisible" class="flow-modal-overlay" @click.self="closeIframe">
        <div class="flow-modal">
          <div class="flow-modal-header">
            <span>{{ iframeTitle }}</span>
            <button @click="closeIframe">关闭</button>
          </div>
          <iframe
            :src="iframeUrl"
            class="flow-modal-iframe"
            frameborder="0"
          />
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
/**
 * 核心逻辑已封装在 useFlowApproval composable 中。
 * composable 负责：
 * - 5 个 Tab 的 API 数据请求（待办/已办/待发/已发/超时跳转）
 * - 分页管理
 * - iframe 弹窗状态管理
 * - 操作按钮配置（审核/查看/编辑/催办/撤销/删除/取回/查看流程）
 * - 构建 iframe URL（hash mode: /flow/#/flow/detail/{id}?sid=...&mode=...）
 *
 * 完整实现见: rsh-unified-vue3-web/src/composables/useFlowApproval.ts
 */
import { onMounted } from 'vue'
import {
  useFlowApproval,
  FLOW_TABS,
  getActions,
  type FlowTab,
} from '@/composables/useFlowApproval'

const tabs = FLOW_TABS

const {
  activeTab,
  items,
  loading,
  tabCounts,
  iframeVisible,
  iframeUrl,
  iframeTitle,
  fetchList,
  fetchAllCounts,
  switchTab,
  handleAction,
  closeIframe,
} = useFlowApproval()

onMounted(() => {
  fetchList()
  fetchAllCounts()
})
</script>

<style scoped>
.flow-demo { padding: 20px; }
.tabs { display: flex; gap: 8px; margin-bottom: 16px; }
.tabs button.active { color: #409eff; border-bottom: 2px solid #409eff; }
.list-item { display: flex; align-items: center; gap: 12px; padding: 8px 0; border-bottom: 1px solid #eee; }
.title { flex: 1; }
.actions { display: flex; gap: 4px; }

/* 全屏 iframe 弹窗 */
.flow-modal-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
}
.flow-modal {
  width: 96vw;
  height: 94vh;
  background: #fff;
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.flow-modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid #e5e7eb;
}
.flow-modal-iframe {
  flex: 1;
  width: 100%;
  border: none;
}
</style>
