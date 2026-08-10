<template>
  <el-dialog :visible="visible" title="选择流程" :close-on-click-modal="false" width="550px" @close='handleClose'
    class="examiner-dialog" append-to-body>
      <el-radio-group v-model="flowRadio">
        <el-radio v-for="item in flowList" :label="item.id" style="margin-bottom:10px">{{ item.flowName }}</el-radio>
        <!-- <el-radio :label="6">备选项</el-radio>
        <el-radio :label="9">备选项</el-radio> -->
      </el-radio-group>
    <span slot="footer">
      <el-button @click="handleClose">取 消</el-button>
      <el-button type="primary" @click="confirm">确 定</el-button>
    </span>
  </el-dialog>
</template>

<script>
export default {
  name:'',
  components: {},
  props: ['visible','flowList'],
  data() {
    return {
      flowRadio:'',
    };
  },
  created() {},
  mounted() {},
  watch: {},
  computed: {},
  methods: {
    handleClose(){
      this.$emit('update:visible', false);
    },
    confirm(){
      this.$emit('confirm',this.flowRadio)
      this.handleClose()
    }
  },
};
</script>
<style lang="scss" scoped>
</style>
