<!--
 * @Descripttion: your project
 * @Author: zhengzetao
 * @Date: 2022-07-30 10:24:56
-->
<template>
  <el-dialog :title="modifyDialogTitle" :visible.sync="visible" width="40%" :close-on-click-modal="false"
    @close='handleClose'>
    <el-form :model="nodeTreeForm" ref="nodeTreeForm" :rules="nodeTreeRules" label-width="100px">
      <el-form-item label="名称：" prop="name">
        <el-input type="input" v-model="nodeTreeForm.name"></el-input>
      </el-form-item>
    </el-form>
    <span slot="footer" class="dialog-footer">
      <el-button @click="handleClose">取 消</el-button>
      <el-button type="primary" @click="modifynodeTree('nodeTreeForm')">确 定</el-button>
    </span>
  </el-dialog>
</template>

<script>
export default {
  name: '',
  components: {},
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    modifyDialogTitle: {
      type: String,
      default: ''
    }
  },
  data() {
    return {
      nodeTreeForm: {
        name: ''
      },
      nodeTreeRules: {
        name: [
          { required: true, max: 64, message: '请输入名称', trigger: 'blur' }
        ]
      }
    };
  },
  computed: {},
  watch: {},
  created() { },
  mounted() { },
  methods: {
    modifynodeTree(formName) {
      this.$refs[formName].validate((valid) => {
        if (valid) {
          this.$emit('updateBudgetChildList', this.nodeTreeForm);
          this.handleClose();
        } else {
          return false;
        }
      });
    },
    handleClose() {
      this.$emit('update:visible', false);
    }
  }
};

</script>
<style lang='scss' scoped>
</style>
