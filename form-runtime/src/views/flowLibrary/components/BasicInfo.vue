<!--
 * @Author: your name
 * @Date: 2021-06-24 19:52:05
 * @LastEditTime: 2021-06-25 14:13:53
 * @LastEditors: Please set LastEditors
 * @Description: In User Settings Edit
 * @FilePath: \rsh-workflow-engine\src\views\flowLibrary\components\basicInfo.vue
-->
<template>
  <div class="main">
    <el-form ref="ruleForm" :model="infoForm" :rules="rules" label-width="160px">
      <el-row>
        <el-col :span="8">
          <el-form-item label="流程名称" prop="flowName">
            <el-input v-model="infoForm.flowName" placeholder="请输入流程名称" :disabled="type == 1" />
          </el-form-item>
        </el-col>
        <!-- <el-col :span="8">
          <el-form-item label="流程分组" prop="flowName">
            <el-input v-model="infoForm.flowName" placeholder="请选择分组" :disabled="type == 1"></el-input>
          </el-form-item>
        </el-col> -->
        <el-col :span="8">
          <el-form-item label="流程分组" prop="groupId">
            <el-select v-model="infoForm.groupId" placeholder="请选择流程分组" :disabled="type == 1">
              <el-option v-for="item in flowList" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="8">
          <el-form-item label="用途说明" prop="remark">
            <el-input
              v-model="infoForm.remark"
              type="textarea"
              :rows="3"
              placeholder="请输入用途说明"
              :disabled="type == 1"
            />
          </el-form-item>
        </el-col>
      </el-row>
    </el-form>
    <div class="btn-box">
      <el-button type="primary" class="btn" @click="handleNext">下一步</el-button>
    </div>
  </div>
</template>
<script>
import Bus from '@/utils/bus.js';
export default {
  name: '',
  components: {},
  props: {
    infoForm: {
      type: Object,
      default: () => {
        return {
          flowName: '',
          typeId: '',
          remark: '',
          groupId: ''
        };
      }
    },
    flowList: {
      type: Array,
      default: () => {
        return [];
      }
    }
  },
  data() {
    return {
      type: '',
      rules: {
        flowName: [
          { required: true, message: '请输入流程名称', trigger: 'blur' }
        ],
        remark: [
          { required: true, message: '请输入用途说明', trigger: 'blur' }
        ]
      }
    };
  },
  computed: {},
  watch: {},
  created() {
    this.type = this.$route.query.type;
  },
  mounted() { },
  methods: {
    handleNext() {
      this.$refs.ruleForm.validate((valid) => {
        if (valid) {
          this.$emit('updateActive', 1);
          Bus.$emit('sendInfoForm', this.infoForm);
        } else {
          this.$message.error('缺少必填的数据或数据格式不正确');
        }
      });
    }
  }
};
</script>
<style scoped lang='scss'>
.main {
  position: relative;
  width: 100%;
  height: 600px;
  background-color: #fff;
}

.btn-box {
  width: 100%;
  position: absolute;
  text-align: center;
  bottom: 60px;
  text-align: center;
}
</style>
