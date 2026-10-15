<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  NCard,
  NIcon,
  NSpin,
  NTag,
} from 'naive-ui'
import {
  ArrowForwardOutline,
  CheckmarkCircleOutline,
  GitBranchOutline,
  LayersOutline,
  PersonRemoveOutline,
  ShieldCheckmarkOutline,
} from '@vicons/ionicons5'

import { fetchRuntimeSource, type RuntimeSourceState } from '../features/form-runtime-maintenance/api'
import { fetchAddSignExclusions } from '../features/add-sign-exclusion/api'

const router = useRouter()

// 异步概览数据：用于在设置主页展示关键指标摘要
const loadingStats = ref(true)
const runtimeSource = ref<RuntimeSourceState | null>(null)
const exclusionCount = ref<number | null>(null)

// 导航跳转辅助
function navigateTo(path: string) {
  void router.push(path)
}

// 加载概览统计（容错处理，接口不可用时不阻塞页面正常展示）
async function loadOverviewStats() {
  loadingStats.value = true
  try {
    const [sourceRes, exclusionRes] = await Promise.allSettled([
      fetchRuntimeSource(),
      fetchAddSignExclusions(),
    ])
    if (sourceRes.status === 'fulfilled') {
      runtimeSource.value = sourceRes.value
    }
    if (exclusionRes.status === 'fulfilled') {
      exclusionCount.value = exclusionRes.value.total ?? exclusionRes.value.items?.length ?? 0
    }
  }
  finally {
    loadingStats.value = false
  }
}

onMounted(() => {
  void loadOverviewStats()
})
</script>

<template>
  <div class="settings-container">
    <!-- 头部区域：页面定位与安全声明 -->
    <header class="settings-header">
      <div class="settings-header__info">
        <div class="settings-header__badges">
          <n-tag size="small" :bordered="false" type="info" class="header-badge">
            <template #icon>
              <n-icon :component="ShieldCheckmarkOutline" />
            </template>
            受控隔离运行时
          </n-tag>
          <n-tag size="small" :bordered="false" type="success" class="header-badge">
            <template #icon>
              <n-icon :component="CheckmarkCircleOutline" />
            </template>
            零目标平台配置侵入
          </n-tag>
        </div>
        <h1 class="settings-title">系统设置</h1>
        <p class="settings-subtitle">
          配置本地自动化执行运行时与流程防错名单。所有维护操作均在独立服务与本地持久层生效，不修改目标平台任何系统配置。
        </p>
      </div>
    </header>

    <!-- 主功能模块卡片区域 -->
    <section class="settings-grid">
      <!-- 卡片 1：表单运行时维护 -->
      <div class="setting-card-wrapper" @click="navigateTo('/settings/form-runtime')">
        <n-card class="setting-card setting-card--primary" hoverable>
          <div class="setting-card__body">
            <div class="setting-card__header">
              <div class="setting-icon-box setting-icon-box--blue">
                <n-icon :size="28" :component="LayersOutline" />
              </div>
              <div class="setting-card__title-group">
                <div class="setting-card__meta-tag">Form Runtime</div>
                <h2 class="setting-card__title">表单运行时维护</h2>
              </div>
            </div>

            <p class="setting-card__desc">
              管理本地独立表单渲染与执行运行时。支持受控拉取固定参考仓库源码、原样同步、候选版本校验构建及热重载切换，保障流程执行与目标平台原生渲染逻辑的一致性。
            </p>

            <div class="setting-card__status-bar">
              <n-spin :show="loadingStats" size="small">
                <div v-if="runtimeSource" class="status-summary">
                  <span class="status-chip">
                    <n-icon :component="GitBranchOutline" class="chip-icon" />
                    <span>{{ runtimeSource.branch || '固定来源' }}</span>
                  </span>
                  <span class="status-divider">/</span>
                  <span class="status-chip code-font">
                    {{ runtimeSource.head ? runtimeSource.head.slice(0, 7) : 'HEAD' }}
                  </span>
                  <n-tag
                    size="small"
                    :type="runtimeSource.dirty ? 'error' : 'success'"
                    :bordered="false"
                    class="ml-auto"
                  >
                    {{ runtimeSource.dirty ? '工作树有修改' : '工作树干净' }}
                  </n-tag>
                </div>
                <div v-else-if="!loadingStats" class="status-summary status-summary--placeholder">
                  <span>从固定参考仓库受控原样同步</span>
                  <n-tag size="small" type="info" :bordered="false" class="ml-auto">点击进入</n-tag>
                </div>
              </n-spin>
            </div>

            <div class="setting-card__footer">
              <span class="setting-card__action-text">进入表单运行时维护</span>
              <div class="setting-card__action-btn">
                <n-icon :size="18" :component="ArrowForwardOutline" />
              </div>
            </div>
          </div>
        </n-card>
      </div>

      <!-- 卡片 2：加签人员排除名单 -->
      <div class="setting-card-wrapper" @click="navigateTo('/settings/add-sign-exclusion')">
        <n-card class="setting-card setting-card--amber" hoverable>
          <div class="setting-card__body">
            <div class="setting-card__header">
              <div class="setting-icon-box setting-icon-box--amber">
                <n-icon :size="28" :component="PersonRemoveOutline" />
              </div>
              <div class="setting-card__title-group">
                <div class="setting-card__meta-tag">Exclusion List</div>
                <h2 class="setting-card__title">人员排除</h2>
              </div>
            </div>

            <p class="setting-card__desc">
              维护在一键配置加签、移交动作时自动排除的人员清单。防止算法随机选中「组织目录中虽有账号但无法在目标平台正常登录」的人员，避免执行节点因无法登录而停机失败。
            </p>

            <div class="setting-card__status-bar">
              <n-spin :show="loadingStats" size="small">
                <div v-if="exclusionCount !== null" class="status-summary">
                  <span class="status-chip">
                    已排除人员：<strong>{{ exclusionCount }}</strong> 人
                  </span>
                  <n-tag size="small" type="warning" :bordered="false" class="ml-auto">
                    自动防错生效中
                  </n-tag>
                </div>
                <div v-else-if="!loadingStats" class="status-summary status-summary--placeholder">
                  <span>自动规避不可用加签人员</span>
                  <n-tag size="small" type="warning" :bordered="false" class="ml-auto">点击管理</n-tag>
                </div>
              </n-spin>
            </div>

            <div class="setting-card__footer">
              <span class="setting-card__action-text">管理人员排除</span>
              <div class="setting-card__action-btn">
                <n-icon :size="18" :component="ArrowForwardOutline" />
              </div>
            </div>
          </div>
        </n-card>
      </div>
    </section>
  </div>
</template>

<style scoped>
.settings-container {
  max-width: 1080px;
  margin: 0 auto;
  padding: 8px 0 40px;
  display: flex;
  flex-direction: column;
  gap: 28px;
}

/* 头部样式 */
.settings-header {
  padding: 8px 4px 4px;
}

.settings-header__badges {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.header-badge {
  font-weight: 500;
  border-radius: 6px;
  padding: 2px 10px;
}

.settings-title {
  margin: 0 0 10px;
  font-size: 28px;
  font-weight: 700;
  line-height: 1.25;
  letter-spacing: -0.01em;
  color: var(--text-color-1);
}

.settings-subtitle {
  margin: 0;
  font-size: 14.5px;
  line-height: 1.65;
  color: var(--text-color-3);
  max-width: 820px;
}

/* 主卡片网格 */
.settings-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 24px;
}

.setting-card-wrapper {
  cursor: pointer;
  height: 100%;
}

.setting-card {
  height: 100%;
  border-radius: 14px;
  transition: color 160ms var(--ta-ease), background-color 160ms var(--ta-ease), border-color 160ms var(--ta-ease), box-shadow 160ms var(--ta-ease);
  position: relative;
  overflow: hidden;
}

.setting-card:hover {

  box-shadow: var(--ta-shadow-1);
}

.setting-card--primary:hover {
  border-color: var(--primary-color);
}

.setting-card--amber:hover {
  border-color: var(--warning-color);
}

.setting-card__body {
  display: flex;
  flex-direction: column;
  height: 100%;
  gap: 16px;
  padding: 6px 4px;
}

.setting-card__header {
  display: flex;
  align-items: center;
  gap: 16px;
}

.setting-icon-box {
  width: 54px;
  height: 54px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.setting-icon-box--blue {
  background: color-mix(in srgb, var(--info-color) 12%, transparent);
  color: var(--info-color);
}

.setting-icon-box--amber {
  background: color-mix(in srgb, var(--warning-color) 14%, transparent);
  color: var(--warning-color);
}

.setting-card__title-group {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.setting-card__meta-tag {
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--text-color-3);
}

.setting-card__title {
  margin: 0;
  font-size: 19px;
  font-weight: 600;
  color: var(--text-color-1);
  line-height: 1.3;
}

.setting-card__desc {
  margin: 0;
  font-size: 13.5px;
  line-height: 1.6;
  color: var(--text-color-2);
  flex-grow: 1;
}

/* 状态概览条 */
.setting-card__status-bar {
  background: var(--ta-surface-2);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 8px 12px;
  min-height: 42px;
  display: flex;
  align-items: center;
}

.status-summary {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12.5px;
  color: var(--text-color-2);
}

.status-summary--placeholder {
  color: var(--text-color-3);
}

.status-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.chip-icon {
  font-size: 14px;
}

.status-divider {
  color: var(--text-color-3);
  font-weight: 300;
}

.code-font {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.ml-auto {
  margin-left: auto;
}

/* 操作入口按钮区 */
.setting-card__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-top: 4px;
  border-top: 1px dashed var(--border-color);
}

.setting-card__action-text {
  font-size: 13.5px;
  font-weight: 500;
  color: var(--text-color-2);
  transition: color 0.15s ease;
}

.setting-card:hover .setting-card__action-text {
  color: var(--primary-color);
}

.setting-card__action-btn {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: var(--ta-surface-2);
  border: 1px solid var(--border-color);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-color-2);
  transition: color 160ms var(--ta-ease), background-color 160ms var(--ta-ease), border-color 160ms var(--ta-ease), box-shadow 160ms var(--ta-ease);
}

.setting-card:hover .setting-card__action-btn {
  background: var(--primary-color);
  color: var(--ta-on-primary);
  border-color: var(--primary-color);

}

/* 响应式适配 */
@media (max-width: 820px) {
  .settings-grid {
    grid-template-columns: 1fr;
  }

  .settings-container {
    padding: 0 4px 32px;
  }
}
</style>
