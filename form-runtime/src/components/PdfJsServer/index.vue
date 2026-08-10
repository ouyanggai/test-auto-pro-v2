<!--
 * @Descripttion:
 * @Author: Calvin
 * @Date: 2021-05-10 10:26:28
-->
<template>
  <div class="pdf_page">
    <iframe width="100%" :height="height" scrolling="no" :src="serverUrl"></iframe>
  </div>
</template>

<script>
export default {
  props: {
    serverUrl: {
      type: String,
      default: ''
    },
    height: {
      type: String,
      default: '700'
    }
  },
  data() {
    return {
    };
  },
  mounted() {
  }
};
</script>

<style>

</style>
