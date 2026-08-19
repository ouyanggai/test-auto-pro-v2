<template>
  <section class="placeholder-page">
    <h1>系统设置</h1>
    <p>维护工具只操作本项目运行时，不修改目标平台。</p>
    <nav class="settings-links" aria-label="系统设置">
      <router-link to="/settings/form-runtime">表单运行时维护</router-link>
      <router-link to="/settings/template-rules">模板规则目录</router-link>
    </nav>
  </section>
</template>

<style scoped>
.settings-links { display: flex; gap: 16px; flex-wrap: wrap; }
</style>
