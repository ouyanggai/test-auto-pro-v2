<script setup lang="ts">
import { NButton, NPopover, NScrollbar } from 'naive-ui'
import { HelpCircleOutline } from '@vicons/ionicons5'

import { reportTermGroups, reportTermsByIds } from './glossary'

// ReportGlossaryEntry 是三个页面右上角共用的「指标口径说明」总入口：
// 点开按分组列出本级涉及的全部名词，与行内 info 图标共用同一份术语表。
// 只列本级名词：L0 列全部，L1/L2 只列本级看得到的，用户不用翻无关口径。
const props = defineProps<{ termIds: string[] }>()

// groupedTerms 按固定分组顺序装配本级名词；某一组在本级没有名词时整组不渲染。
const groupedTerms = reportTermGroups
  .map((group) => ({ title: group.title, terms: reportTermsByIds(group.ids.filter((id) => props.termIds.includes(id))) }))
  .filter((group) => group.terms.length > 0)
</script>

<template>
  <NPopover trigger="click" placement="bottom-end" :show-arrow="true" class="glossary-entry__popover">
    <template #trigger>
      <NButton size="small" quaternary class="glossary-entry__trigger">
        <template #icon>
          <HelpCircleOutline />
        </template>
        指标口径说明
      </NButton>
    </template>
    <NScrollbar class="glossary-entry__scroll">
      <section class="glossary-entry">
        <p class="glossary-entry__lead">这里用大白话解释本页每个数字怎么算、分母是什么、数据来自哪里。看不懂的名词，先看这里。</p>
        <section v-for="group in groupedTerms" :key="group.title" class="glossary-entry__group">
          <h3 class="glossary-entry__title">{{ group.title }}</h3>
          <dl class="glossary-entry__list">
            <template v-for="term in group.terms" :key="term.id">
              <dt>{{ term.name }}</dt>
              <dd>
                <p class="glossary-entry__summary">{{ term.summary }}</p>
                <p class="glossary-entry__meta">怎么算：{{ term.formula }}</p>
                <p class="glossary-entry__meta">数据来自哪里：{{ term.source }}</p>
                <p class="glossary-entry__meta">容易误解的点：{{ term.caveat }}</p>
              </dd>
            </template>
          </dl>
        </section>
      </section>
    </NScrollbar>
  </NPopover>
</template>

<style scoped>
.glossary-entry__scroll {
  max-height: 60vh;
  max-width: min(560px, 90vw);
}
.glossary-entry {
  display: grid;
  gap: 14px;
  padding-right: 8px;
  line-height: 1.7;
}
.glossary-entry__lead {
  margin: 0;
  color: var(--text-color-3);
  font-size: 12px;
}
.glossary-entry__group {
  display: grid;
  gap: 6px;
}
.glossary-entry__title {
  margin: 0;
  font-size: 13px;
  color: var(--primary-color);
}
.glossary-entry__list {
  display: grid;
  grid-template-columns: 96px 1fr;
  gap: 6px 12px;
  margin: 0;
  font-size: 12px;
}
.glossary-entry__list dt {
  color: var(--text-color-1);
  font-weight: 500;
}
.glossary-entry__list dd {
  margin: 0;
}
.glossary-entry__summary {
  margin: 0 0 2px;
}
.glossary-entry__meta {
  margin: 0;
  color: var(--text-color-3);
}
</style>
