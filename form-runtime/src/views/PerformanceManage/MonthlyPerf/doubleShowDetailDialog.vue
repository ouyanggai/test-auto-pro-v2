<template>
  <el-dialog
    :visible="visible"
    :title="value.title"
    :close-on-click-modal="false"
    :width="'60%'"
    :append-to-body="true"
    @close="_=>{this.$emit('update:visible', false)}"
  >
    <div class="dialog-container">
      <pre style="font:unset;white-space:pre-line;line-height:30px;">{{ value.val }}</pre>
    </div>
    <span slot="footer">
      <el-button @click="_=>{this.$emit('update:visible', false)}">关 闭</el-button>
      <!-- <el-button type="primary" @click="confirmClick">确定</el-button> -->
    </span>
  </el-dialog>
</template>
<script>
export default {
  name: '',
  components: { },
  props: ['visible', 'value'],
  data() {
    return {
    };
  },
  computed: {
  },
  watch: {},
  created() {},
  mounted() {},
  methods: {
    handleClose() {
      this.$emit('update:visible', false);
    },
    confirmClick() {
      this.handleClose();
    }
  }
};

</script>
<style lang='scss' scoped>
.dialog-container {
}
</style>
