<!--
 * @Descripttion:重定向模板
 * @Author: Calvin
 * @Date: 2021-06-03 11:04:35
-->
<template>
  <div class="redirect-container">
    <!-- <router-view /> -->
    <!-- <keep-alive :include="keepAlivePage">
      <router-view :key="key" />
    </keep-alive> -->
    <keep-alive>
      <router-view v-if="$route.meta.keepAlive" :key="key"></router-view>
    </keep-alive>
    <router-view v-if="!$route.meta.keepAlive" :key="key"></router-view>
  </div>
</template>

<script>
import { mapGetters } from 'vuex';
export default {
  name: '',
  components: {},
  data() {
    return {
    };
  },
  computed: {
    ...mapGetters([
      'keepAlivePage'
    ]),
    key(){
      return this.$route.path
    }
  },
  watch: {},
  created() { },
  mounted() {
    // console.log('keepAlivePage', this.keepAlivePage);
  },
  methods: {}
};
</script>

<style scoped lang="scss">
.redirect-container {
  width: 100%;
  height: 100%;
  overflow-y: auto;
}
</style>
