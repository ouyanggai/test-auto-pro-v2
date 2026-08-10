<!--
 * @Descripttion: 项目立项记录
 * @Author: liufuze
-->
<template>
  <el-dialog title="查看立项记录" :visible.sync="logVisible" :before-close="closeDialog" :append-to-body="true" width="750px">
    <div style="max-height: 45vh;overflow: auto;">
      <el-table>
        <el-table-column type="index" label="序号" width="60">
        </el-table-column>
        <el-table-column label="流程名称">
        </el-table-column>
        <el-table-column label="项目预算金额/追加金额（万）" width="220">
        </el-table-column>
        <el-table-column label="申请人">
        </el-table-column>
        <el-table-column label="提交申请时间">
        </el-table-column>
        <el-table-column label="查看" width="60">
        </el-table-column>
      </el-table>
    </div>
    <span slot="footer" class="dialog-footer">
      <el-button @click="closeDialog">关 闭</el-button>
    </span>
  </el-dialog>
</template>

<script>
export default {
  name: 'ProjBudgetLog',
  props: ['logVisible'],
  data() {
    return {
    };
  },
  watch: {},
  computed: {},
  methods: {},
  created() { },
  mounted() { },
  methods: {
    closeDialog() {
      this.$emit("update:logVisible", false)
    }
  }
};
</script>
<style lang="scss" scoped></style>
