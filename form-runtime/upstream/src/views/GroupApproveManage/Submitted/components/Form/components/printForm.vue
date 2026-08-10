<!--
 * @Descripttion: 使用配置生成表单
 * @Author: liufuze
 -->
<template>
  <div class="form-printForm">
    <span>{{ formTitle }}</span>
    <div style="margin-top: 15px;">
      <el-form ref="form" :model="formData" :label-width="labelWidth" label-suffix="：">
        <el-row v-for="(item, index) in formList" :key="index">
          <el-col v-for="(col, k) in item" :key="k" :span="col.span || 12">
            <el-form-item :label="col.label" :prop="col.prop">
              <span>{{ col.value }}</span>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </div>
  </div>
</template>
<script>
export default {
  name: 'PrintForm',
  props: {
    formList: {
      type: Array,
      default() {
        return [];
      }
    },
    formTitle: {
      type: String,
      default: ''
    },
    labelWidth: {
      type: String,
      default: '140px'
    }
  },
  data() {
    return {
      formData: {}
    };
  },

  methods: {
  }
};
</script>
<style scoped lang="scss">
.form-printForm {
  width: 100%;
  box-sizing: border-box;
  // border: 1px solid #e2e2e2;

}

label {
  font-weight: normal !important;
}

::v-deep .el-input--mini,
::v-deep .el-input-number--mini {
  width: 250px;
}
</style>
<style lang="scss">
.form-printForm {
  label {
    font-weight: normal !important;
  }

  .el-form-item {
    margin-bottom: 10px;
  }
}
</style>
