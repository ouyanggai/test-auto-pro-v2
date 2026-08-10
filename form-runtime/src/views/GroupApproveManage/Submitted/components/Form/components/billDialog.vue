<!--
 * @Descripttion: 借款单/请款单
 * @Author: lby
-->
<template>
  <el-dialog :title="'请选择' + billType + '单'" :visible.sync="billVisible" :before-close="closeDialog"
    :append-to-body="true" width="750px">
    <div style="max-height: 45vh;overflow: auto;">
      <el-table :data="tableData" @selection-change="handleSelectionChange">
        <el-table-column type="selection" width="60" align="center"></el-table-column>
        <el-table-column label="流程名称" prop="name">
        </el-table-column>
        <el-table-column label="借款金额（元）" width="220">
        </el-table-column>
        <el-table-column label="申请人">
        </el-table-column>
        <el-table-column label="提交申请时间">
        </el-table-column>
        <el-table-column label="查看" width="60">
          <template slot-scope="scope">
            <el-button @click="handleClick(scope.row)" type="text" size="small">详情</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
    <span slot="footer" class="dialog-footer">
      <el-button @click="closeDialog">关 闭</el-button>
      <el-button type="primary" @click="sure">确 定</el-button>
    </span>
  </el-dialog>
</template>

<script>
export default {
  name: 'ProjBudgetLog',
  props: {
    billVisible: {
      type: Boolean,
      default: false
    },
    billType: {
      type: String,
      default: ''
    }
  },
  data() {
    return {
      tableData: [
        { name: '11111' },
        { name: '2222' },
        { name: '3333' }
      ],
      multipleSelection: []
    };
  },
  watch: {},
  computed: {},
  created() { },
  mounted() { },
  methods: {
    handleClick(row) {

    },
    handleSelectionChange(val) {
      this.multipleSelection = val;
    },
    closeDialog() {
      this.$emit('update:billVisible', false);
    },
    sure() {
      if (this.multipleSelection.length != 1) {
        this.$message.error('请选择单条数据');
        return;
      }
      this.$emit('selectData', this.multipleSelection[0]);
      this.closeDialog();
    }
  }
};
</script>
<style lang="scss" scoped></style>
