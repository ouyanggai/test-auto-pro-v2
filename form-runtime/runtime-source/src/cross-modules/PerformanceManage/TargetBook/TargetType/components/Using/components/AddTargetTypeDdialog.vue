<!--
 * @Descripttion:组织架构 / 架构信息 / 架构设置
 * @Author: Calvin
 * @Date: 2021-06-03 11:07:32
-->
<template>
  <el-dialog
    v-loading="loading"
    :title="title"
    :visible="visible"
    :close-on-click-modal="false"
    @close='handleClose'
  >
    <el-form
      :model="addTypeForm"
      ref="addTypeForm"
      :rules="rules"
    >
      <el-form-item
        label="目标名称"
        :label-width="formLabelWidth"
        prop="name"
      >
        <el-input v-model.trim="addTypeForm.name" maxlength="50"></el-input>
      </el-form-item>
      <el-form-item
        label="目标类型"
        :label-width="formLabelWidth"
        prop="manageType"
      >
        <el-select v-model="addTypeForm.manageType">
          <el-option
            label="工作指标"
            value="work_target"
          ></el-option>
          <el-option
            label="管理指标"
            value="manager_target"
          ></el-option>
        </el-select>
      </el-form-item>

      <el-form-item
        label="说明"
        :label-width="formLabelWidth"
      >
        <el-input
          type="textarea"
          :autosize="{ minRows: 2, maxRows: 4}"
          maxlength="100"
          show-word-limit
          resize="none"
          v-model="addTypeForm.remarks"
        ></el-input>
      </el-form-item>
    </el-form>
    <div
      slot="footer"
      class="dialog-footer"
    >
      <el-button @click="handleClose">取 消</el-button>
      <el-button
        type="primary"
        @click="submitForm('addTypeForm')"
      >确 定</el-button>
    </div>
  </el-dialog>

</template>

<script>
import Api from '@/api';
export default {
  name: '',
  components: {},
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    id: {
      type: String,
      default: ''
    },
    title: {
      type: String,
      default: ''
    }
  },
  data() {
    return {
      loading: false,
      formLabelWidth: '120px',
      addTypeForm: {
        manageType: '',
        name: '',
        remarks: ''
      },
      rules: {
        manageType: [
          {
            required: true,
            message: '请选择目标类型',
            trigger: 'change'
          }
        ],
        name: [
          {
            required: true,
            message: '请输入目标名称',
            trigger: 'blur'
          }
        ]
      }
    };
  },
  computed: {},
  watch: {},
  created() { },
  mounted() {
    if (this.id) {
      this.findWorkTargetType();
    }
  },
  updated() { },
  methods: {
    findWorkTargetType() {
      this.loading = true;
      this.$axios.post(
        Api.performance.indicatorsTypeDetail,
        {
          data: { id: this.id }
        },
        res => {
          this.loading = false;
          if (res.isSuccess) {
            const {
              manageType,
              name,
              remarks
            } = res.data;
            this.addTypeForm = {
              manageType,
              name,
              remarks
            };
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    handleClose() {
      this.$emit('update:visible', false);
    },
    submitForm(formName) {
      this.$refs[formName].validate(valid => {
        if (valid) {
          let postUrl = Api.performance.indicatorsTypeSave;
          const data = { ...this.addTypeForm };
          if (this.id) {
            data.id = this.id;
            postUrl = Api.performance.indicatorsTypeUpdate;
          }
          this.loading = true;
          this.$axios.post(
            postUrl,
            {
              data
            },
            res => {
              this.loading = false;
              if (res.isSuccess) {
                this.$message.success('保存成功');
                this.$emit('success');
                this.$emit('update:visible', false);
              } else {
                this.$message.error(res.message);
              }
            }
          );
        } else {
          return false;
        }
      });
    }
  }
};
</script>

<style scoped lang="scss">
</style>
