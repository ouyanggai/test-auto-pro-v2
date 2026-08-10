<template>
  <el-dialog
    title="日志详情"
    :visible.sync="dialogVisible"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
    :before-close="handleClose"
    width="800px"
    >
    <div style="min-height: 560px;overflow: auto;" v-if="rowData">
      <JsonViewer
        :value="rowData"
        :font-size="16"
        :expand-depth="4"
        :copyable="copyable"
        :expanded="true"
        ></JsonViewer>
    </div>
    <div v-else style="text-align: center;line-height: 400px;">
      <p>暂无数据</p>
    </div>
  </el-dialog>
</template>
<script>
import JsonViewer from 'vue-json-viewer';
export default {
  components: { JsonViewer },
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    rowData: {
      type: [Object, String],
      default: () => ''
    }
  },
  watch: {
    visible(val) {
      this.dialogVisible = val;
    }
  },
  computed: {
  },
  mounted() {
    this.dialogVisible = this.visible;
  },
  data() {
    return {
      dialogVisible: false,
      copyable: {
        copyText: '复制', copiedText: '复制完成', timeout: 2000
      }
    };
  },
  methods: {
    handleClose(done) {
      this.$emit('update:visible', false);
      done();
    }
  }
};
</script>
<style lang="scss" scoped>
::v-deep .el-dialog.is-fullscreen .el-dialog__body{
  max-height: 100vh !important;
  height: calc(100vh - 60px);
  padding: 0 !important;

}
::v-deep .el-dialog__footer{
  text-align: center !important;
}
::v-deep .el-table__fixed-right::before{
  background-color: #ffffff !important;

}
</style>
