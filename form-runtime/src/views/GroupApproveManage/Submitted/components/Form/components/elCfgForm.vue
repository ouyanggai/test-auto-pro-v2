<!--
 * @Descripttion: 使用配置生成表单
 * @Author: liufuze
 -->
<template>
  <div class="form-container">
    <span>{{ formTitle }}</span>
    <div style="margin-top: 15px;">
      <el-form ref="form" :model="formData" :label-width="labelWidth" :rules="rule" v-if="formSourceList.length">
        <el-row v-for="item in formSourceList">
          <el-col v-for="col in item" :span="col.span || 12">
            <el-form-item :label="col.label" :prop="col.prop">
              <template v-if="col.nodeType == 'select'">
                <el-select v-if="col.changeEvent" v-model="formData[col.prop]" :disabled="col.disabled"
                  @change="col.changeEvent">
                  <el-option v-for="el in col.data" :label="el.label || el.name" :value="el.value || el.id"></el-option>
                </el-select>
                <el-select v-else v-model="formData[col.prop]" :disabled="col.disabled">
                  <el-option v-for="el in col.data" :label="el.label || el.name" :value="el.value || el.id"></el-option>
                </el-select>
                <el-link type="primary" v-if="col.isTextButton" style="margin-left: 5px;"
                  @click="showDialog">立项记录</el-link>
              </template>
              <el-input v-if="col.nodeType == 'input'" :disabled="col.disabled" v-model="formData[col.prop]"></el-input>
              <el-input-number v-if="col.nodeType == 'number'" :disabled="col.disabled" :controls="col.controls"
                :min="col.min" v-model="formData[col.prop]"></el-input-number>
              <el-input v-if="col.nodeType == 'text'" type="textarea"
                :autosize="{ minRows: col['minRows'] || 2, maxRows: col['maxRows'] || 4 }" placeholder="请输入内容"
                :disabled="col.disabled" v-model="formData[col.prop]" :maxlength="col.limit" show-word-limit>
              </el-input>
              <el-radio-group v-model="formData[col.prop]" v-if="col.nodeType == 'radio'" :disabled="col.disabled">
                <el-radio v-for="el in col.data" :label="el.label">{{ el.value }}</el-radio>
              </el-radio-group>
              <el-date-picker :type="col.type" :picker-options="col.pickerOptions ? col.pickerOptions : ''"
                :format="col.format || 'yyyy-MM-dd'" :value-format="col.format || 'yyyy-MM-dd'"
                v-model="formData[col.prop]" v-if="col.nodeType == 'date-picker'">
              </el-date-picker>
            </el-form-item>
            <el-form-item v-if="col.nodeType == 'upload'">
              <elupload ref="eleupload"></elupload>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </div>
    <projBudgetLog :logVisible.sync="logVisible"></projBudgetLog>
  </div>
</template>
<script>
import elupload from '@/components/EleUpload';
import { deepClone } from '@/utils';
import projBudgetLog from './projBudgetLog';
export default {
  name: 'ElCfgForm',
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
  components: { elupload, projBudgetLog },
  data() {
    return {
      rule: {},
      formData: {
      },
      formSourceList: [],
      logVisible: false
    }
  },
  watch: {
    formList: {
      handler(val) {
        this.formSourceList = val;
        this.genRule();
      },
      // immediate: true,
      deep: true
    }
  },
  created() {
    this.formSourceList = deepClone(this.formList);
    this.genRule();
  },
  methods: {
    genRule() {
      // console.log('genRule', this.formSourceList, this.formData)
      if (this.formSourceList.length) {
        var rule = {}; var formData = {};
        this.formSourceList.forEach(el => {
          el.forEach(item => {
            if (item.prop) formData[item.prop] = item.value || '';
            if (item.isRequire === true) {
              const tempArr = [];
              let message, trigger;
              const inputArr = ['input', 'number', 'text', 'date-picker'];
              if (inputArr.indexOf(item.nodeType) > -1) {
                message = `请输入${item.label}`;
                trigger = 'blur';
              } else {
                message = `请选择${item.label}`;
                trigger = 'change';
              }
              const tempObj = {
                required: true,
                message,
                trigger
              };
              tempArr.push(tempObj);
              rule[item.prop] = tempArr;
            }
          })
        })
        this.rule = deepClone(rule)
        this.formData = deepClone(formData)
        this.$nextTick(() => {
          this.$refs.form.clearValidate()
        })
      }
    },
    getData() {
      var fileArr = this.$refs.eleupload[0].getFileId();
      this.formData.fileIds = fileArr;
      return new Promise((resolve, reject) => {
        this.$refs.form.validate((valid) => {
          if (valid) {
            resolve(this.formData);
          } else {
            setTimeout(() => {
              reject(false);
            });
          }
        });
      });
    },
    showDialog() {
      this.logVisible = true
    }
  }
};
</script>
<style scoped>
.form-container {
  width: 100%;
  padding: 25px;
  box-sizing: border-box;
  border: 1px solid #e2e2e2;

}

::v-deep .el-input--mini,
::v-deep .el-input-number--mini {
  width: 250px;
}
</style>
