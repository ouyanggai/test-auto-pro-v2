<template>
  <div class="targetCheck">
    <el-main>
      <el-tabs v-model="activeName">
        <el-tab-pane label="工作指标" name="work">
          <WorkTarget v-if="activeName == 'work'" />
        </el-tab-pane>
        <el-tab-pane label="管理指标" name="manage">
          <ManageTarget v-if="activeName == 'manage'" />
        </el-tab-pane>
      </el-tabs>
    </el-main>
  </div>
</template>

<script>
import WorkTarget from './components/workTarget.vue';
import ManageTarget from './components/manageTarget.vue';
export default {
  name: 'TargetCheck',
  components: {
    WorkTarget,
    ManageTarget
  },
  data() {
    return {
      activeName: 'work'
    };
  },
  watch: {

  },
  computed: {

  },
  methods: {

  },
  created() {

  },
  mounted() {

  },
  updated() {

  },
  destroyed() {

  }
};
</script>

<style lang="scss" scoped>
.targetCheck{
  height: 100%;
  background: #fff;
}
</style>
