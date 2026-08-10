<!--
 * @description:绩效考核/目标责任书
 * @Author: Calvin
 * @Date: 2022-03-04 09:37:27
 * @FilePath: \src\views\PerformanceManage\TargetBook\index.vue
-->
<template>
  <div class="target-book-container">
    <el-main>
      <el-tabs v-model="activeName">
        <el-tab-pane label="工作指标" name="work">
          <WorkTarget v-if="activeName == 'work'" :kpiScope="kpiScope"/>
        </el-tab-pane>
        <el-tab-pane label="管理指标" name="manage">
          <ManageTarget v-if="activeName == 'manage'" :kpiScope="kpiScope"/>
        </el-tab-pane>
      </el-tabs>
    </el-main>
  </div>
</template>

<script>
import WorkTarget from './components/WorkTarget';
import ManageTarget from './components/ManageTarget';
export default {
  name: '',
  components: { WorkTarget, ManageTarget },
  props: {
    kpiScope: {
      type: [String, Number, undefined],
      default: undefined
    }
  },
  data() {
    return {
      activeName: 'work'
    };
  },
  computed: {},
  watch: {},
  created() {

  },
  activated(){
    this.$bus.$on('targetType', val => {
      if (val == 1) {
        this.activeName = 'work';
      } else {
        this.activeName = 'manage';
      }
    });
  },
  mounted() {
  },
  methods: {}
};
</script>

<style scoped lang="scss">
.target-book-container {
  height: 100%;
  background: #fff;
}
</style>
