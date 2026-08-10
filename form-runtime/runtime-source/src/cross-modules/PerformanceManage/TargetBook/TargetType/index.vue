<!--
 * @Author: junshao
 * @Date: 2022-03-17 11:15:05
 * @LastEditors: junshao
 * @LastEditTime: 2022-03-17 11:16:12
 * @Description: file content
-->
<template>
  <div class="target-type-containner">
    <el-main>
      <el-tabs v-model="activeName">
        <el-tab-pane
          label="使用中"
          name="using"
        >
          <Using v-if="activeName == 'using'" />
        </el-tab-pane>
        <el-tab-pane
          label="已归档"
          name="pigeonhole"
        >
          <Pigeonhole v-if="activeName == 'pigeonhole'" />
        </el-tab-pane>
      </el-tabs>
    </el-main>
  </div>
</template>

<script>
import Using from './components/Using';
import Pigeonhole from './components/Pigeonhole';
export default {
  name: '',
  components: { Pigeonhole, Using },
  data() {
    return {
      activeName: 'using'
    };
  },
  computed: {},
  watch: {},
  created() { },
  mounted() { },
  methods: {
  }
};

</script>
<style lang='scss' scoped>
.target-type-containner {
  height: 100%;
  background: #fff;
}
</style>
