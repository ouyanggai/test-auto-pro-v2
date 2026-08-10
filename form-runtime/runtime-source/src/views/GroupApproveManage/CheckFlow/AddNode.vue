
<!--
 * @Descripttion: 添加节点
 * @Author: zhengzetao
 * @Date: 2021-03-18
-->
<template>
  <div class="add-node-btn-box">
    <div class="add-node-btn">
      <el-popover placement="right-start" v-model="visible">
        <button class="btn" type="button" slot="reference" v-show="editType != 3">
          <span class="iconfont"><i class="el-icon-plus"></i></span>
        </button>
      </el-popover>
    </div>
  </div>
</template>
<script>
export default {
  props: ['childNodeP', 'editType'],

  data() {
    return {
      visible: false
    };
  },
  methods: {
  }
};
</script>
<style lang='scss' scoped>
@import "~@/assets/styles/workflow.scss";
</style>
