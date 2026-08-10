<template>
  <div id="app">
    <router-view />
    <GlobalDialog />
    <GlobalDialog index="2" />
    <GlobalDialog index="3" />
  </div>
</template>

<script>
import GlobalDialog from '@/layout/components/globalDialog/index.vue';

export default {
  name: 'App',
  components: {
    GlobalDialog
  }
};
</script>

<style>
html, body, #app {
  height: 100%;
  margin: 0;
  padding: 0;
}
</style>
