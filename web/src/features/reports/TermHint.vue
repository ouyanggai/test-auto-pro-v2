<script setup lang="ts">
import { NPopover, NText } from 'naive-ui'
import { InformationCircleOutline } from '@vicons/ionicons5'
import { computed } from 'vue'

import { reportTermById } from './glossary'

// TermHint 是指标/表头旁的名词解释入口：点一下弹出「是什么 / 怎么算 / 数据来自哪里 / 容易误解的点」。
// 用户不熟悉测试术语，解释必须就地可查，不用跳页；查不到名词时不渲染图标，避免出现空提示。
const props = defineProps<{ termId: string }>()

const term = computed(() => reportTermById(props.termId))
</script>

<template>
  <NPopover v-if="term" trigger="click" placement="bottom-start" :show-arrow="true" class="term-hint__popover">
    <template #trigger>
      <button type="button" class="term-hint" :aria-label="`名词解释：${term.name}`" :title="`名词解释：${term.name}`">
        <InformationCircleOutline class="term-hint__icon" />
      </button>
    </template>
    <section class="term-hint__body">
      <h3 class="term-hint__title">{{ term.name }}</h3>
      <p class="term-hint__summary">{{ term.summary }}</p>
      <dl class="term-hint__list">
        <dt>怎么算</dt>
        <dd>{{ term.formula }}</dd>
        <dt>数据来自哪里</dt>
        <dd>{{ term.source }}</dd>
        <dt>容易误解的点</dt>
        <dd>{{ term.caveat }}</dd>
      </dl>
      <NText class="term-hint__foot" depth="3">口径与产品说明一致；看不明白的地方以运行详情与日志为准。</NText>
    </section>
  </NPopover>
</template>

<style scoped>
.term-hint {
  display: inline-flex;
  align-items: center;
  padding: 0 2px;
  border: none;
  background: none;
  color: var(--text-color-3);
  cursor: pointer;
  vertical-align: middle;
}
.term-hint:hover,
.term-hint:focus-visible {
  color: var(--primary-color);
}
.term-hint__icon {
  width: 15px;
  height: 15px;
}
.term-hint__body {
  max-width: 340px;
  line-height: 1.7;
}
.term-hint__title {
  margin: 0 0 6px;
  font-size: 14px;
}
.term-hint__summary {
  margin: 0 0 8px;
}
.term-hint__list {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 4px 10px;
  margin: 0;
  font-size: 12px;
}
.term-hint__list dt {
  color: var(--text-color-3);
  white-space: nowrap;
}
.term-hint__list dd {
  margin: 0;
}
.term-hint__foot {
  display: block;
  margin-top: 8px;
  font-size: 12px;
}
</style>
