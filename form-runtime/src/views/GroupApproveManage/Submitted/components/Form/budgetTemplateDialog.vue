<!--
 * @Descripttion: 归口模板，选择列表
 * @Author: liufuze
-->
<template>
  <el-dialog title="归口模板选择" :visible.sync="tempVisible" :before-close="closeTemp" :append-to-body="true" width="400px">
    <div style="overflow: hidden;">
      <el-button type="primary" style="float: right;">应用模板管理<i
          class="el-icon-d-arrow-right el-icon--right"></i></el-button>
    </div>

    <div style="height: 45vh;overflow: auto;">
      <el-tree :data="treeList" show-checkbox node-key="id" :props="defaultProps" :default-expand-all="true">
      </el-tree>
    </div>
    <span slot="footer" class="dialog-footer">
      <el-button @click="closeTemp">取 消</el-button>
      <el-button type="primary">应 用</el-button>
    </span>
  </el-dialog>
</template>
<script>
export default {
  name: 'BudgetTemplateDialog',
  props: ['tempVisible'],
  data() {
    return {
      treeList: [{
        id: 0,
        label: '全选',
        children: [
          {
            id: 1,
            label: '公司固定费用',
            children: [
              { id: 2, label: '打印机费用' },
              { id: 3, label: '零食费用' },
              { id: 4, label: '公共费用' },
            ]
          },
          {
            id: 5,
            label: '开发部',
            children: [
              { id: 6, label: '服务器费用' },
              { id: 7, label: '运维费用' },
              { id: 8, label: '手机费用' },
              { id: 9, label: '网络宽带费用' },
            ]
          },
          {
            id: 6,
            label: '产品部',
            children: [
              { id: 10, label: '服务器费用' },
              { id: 11, label: '运维费用' },
              { id: 12, label: '手机费用' },
              { id: 13, label: '网络宽带费用' },
            ]
          }
        ]
      }],
      defaultProps: {
        children: 'children',
        label: 'label'
      }
    }
  },
  methods: {
    closeTemp() {
      this.$emit("update:tempVisible", false)
    },
  }

}
</script>
<style scoped>
::v-deep .el-dialog__body {
  max-height: 60vh;
  overflow: hidden;
}

::v-deep .el-dialog__footer {
  text-align: center;
}
</style>
