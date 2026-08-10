<!--
 * @Descripttion:创建新流程\基础信息
 * @Author: Calvin
 * @Date: 2021-06-04 11:04:03
-->
<template>
  <div class="info-form-container">
    <el-form ref="formData" :model="formData" :rules="rules" label-width="200px" label-position="top">
      <el-row :gutter="20">
        <el-col :span="12">
          <el-form-item label="流程名称" prop="flowName">
            <el-input v-model="formData.flowName" :disabled="editType == 3" placeholder="请输入流程名称" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="使用范围：">
            <el-select v-model="formData.useScope" placeholder="请选择使用范围" :disabled="editType == 3" @change="changeUseScope">
              <el-option key="invest" label="投资发电平台" value="invest" />
              <el-option key="operation" label="运维平台" value="operation" />
              <el-option key="solar" label="本数光能" value="solar" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="类型" prop="typeId">
            <el-select
              v-model="formData.typeId"
              placeholder="选择类型"
              :disabled="editType == 3||!formData.useScope"
              filterable
              @change="typeChange"
            >
              <el-option v-for="item in formTypeList" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>
          </el-form-item>
        </el-col>
      <!-- </el-row>
      <el-row :gutter="20"> -->
        <el-col :span="12">
          <el-form-item label="流程分组" prop="groupId">
            <el-select v-model="formData.groupId" :disabled="editType == 3||!formData.useScope" placeholder="请选择流程分组" filterable>
              <el-option v-for="item in flowList" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>
          </el-form-item>
        </el-col>
        <template v-if="isRelative">
          <el-col :span="12">
            <el-form-item :label="$route.query.isGeneralFlow ? '选择公司' : '选择相关方公司'" prop="company">
              <el-select v-model="formData.company" :disabled="editType == 3" placeholder="请选择公司" filterable>
                <el-option v-for="item in relativeList" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
        </template>
        <!-- <template v-else>
          <el-col :span="12">
            <el-form-item label="选择公司" prop="company">
              <el-select v-model="formData.company" :disabled="editType == 3" placeholder="请选择公司" filterable>
                <el-option v-for="item in companyList" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
        </template> -->
        <el-col :span="12">
          <el-form-item label="用途说明" prop="remark">
            <el-input
              v-model="formData.remark"
              :disabled="editType == 3"
              type="textarea"
              :rows="3"
              placeholder="请输入用途说明"
            />
          </el-form-item>
        </el-col>
      </el-row>
    </el-form>
  </div>
</template>

<script>
import Api from '@/api';
// import api from '@/api/index';
import Bus from '@/bus';
import { localstorageGet, localstorageSet } from '@/utils/auth';
export default {
  name: 'Steps1',
  components: {},
  props: {
    editType: {
      type: [String, Number],
      default: 1
    },
    infoForm: {
      type: Object,
      default: () => {
        return {
          flowName: '',
          typeId: '',
          groupId: '',
          remark: '',
          company: ''
        };
      }
    },
    companyList: {
      type: Array,
      default: () => {
        return [];
      }
    }
  },
  data() {
    // var validateGroupId = (rule, value, callback) => {
    //   if (this.isCurrentPlatform) {
    //     if (value === '') {
    //       callback(new Error('请选择分组'));
    //     } else {
    //       callback();
    //     }
    //   } else {
    //     callback();
    //   }
    // };
    return {
      // isCurrentPlatform: false, // 是否自建流程
      formData: {
        flowName: '',
        useScope: '',
        typeId: '',
        groupId: '',
        remark: '',
        company: ''
      },
      flowList: [], // 流程分组
      formTypeList: [], // 类型
      rules: {
        flowName: [
          { required: true, message: '请输入流程名称', trigger: 'blur' }
        ],
        // useScope: [
        //   { required: true, message: '请选择使用范围', trigger: 'change' }
        // ],
        typeId: [
          { required: true, message: '请选择类型', trigger: 'change' }
        ],
        groupId: [
          { required: true, message: '请选择流程分组', trigger: 'change' }
        ],
        // company: [
        //   { required: true, message: '请选择公司', trigger: 'change' }
        // ],
        remark: [
          { required: true, message: '请输入用途说明', trigger: 'blur' }
        ]
      },
      step1AuditWay: '',
      fieldTreeList: [],
      isRelative: false,
      relativeList: []
    };
  },
  computed: {},
  watch: {
    infoForm: {
      handler(val) {
        console.log(val, 'val1');
        this.formData = JSON.parse(JSON.stringify(val));
        if (val.typeId) {
          this.getUseScope(val.typeId);
          const formType = this.formTypeList.find(x => x.id == val.typeId);
          if (formType) {
            this.step1AuditWay = formType.auditWay;
            localstorageSet('step1AuditWay', this.step1AuditWay);
            console.log(this.step1AuditWay, 'this.step1AuditWay111');
            this.initFieldList(this.step1AuditWay);
          }
        }
        if (val.useScope) {
          this.getFlowAndType(val.useScope);
        }
      },
      deep: true,
      immediate: true
    }
    // 'infoForm.typeId': {
    //   handler(val) {
    //     console.log(val, 'val1');
    //     if (!val) return false;
    //     // if (!this.infoForm.useScope) {
    //     //   this.getUseScope(val);
    //     // }
    //     const formType = this.formTypeList.find(x => x.id == val);
    //     if (formType) {
    //       this.step1AuditWay = formType.auditWay;
    //       localstorageSet('step1AuditWay', this.step1AuditWay);
    //       this.initFieldList(this.step1AuditWay);
    //     }
    //     // console.log('this.infoForm.typeId:', val);
    //     // this.step1AuditWay = this.formTypeList.find(x => x.id == val).auditWay;
    //     // console.log('this.step1AuditWay:', this.step1AuditWay);
    //     // this.initFieldList(this.step1AuditWay);
    //   },
    //   deep: true,
    //   immediate: true
    // },
    // 'infoForm.useScope': {
    //   handler(val) {
    //     console.log(val, 'val3');
    //     this.changeUseScope(val);
    //   },
    //   deep: true,
    //   immediate: true
    // }
  },
  created() {
    if (this.$route.query.relative) {
      this.isRelative = true;
      if (this.$route.query?.isGeneralFlow) {
        this.getSelectCompanyList();
      } else {
        this.getRelativeList();
      }
    }
  },
  mounted() {
    // this.isCurrentPlatform = this.$route.query.flowCreateType == 'current_platform';
    // this.getFlowGroupList();
    // this.formType();
  },
  methods: {
    typeChange(val){
      const formType = this.formTypeList.find(x => x.id == val);
      if (formType) {
        this.step1AuditWay = formType.auditWay;
        localstorageSet('step1AuditWay', this.step1AuditWay);
        this.initFieldList(this.step1AuditWay);
      }
    },
    // 通过类型id获取使用范围
    getUseScope(val) {
      console.log(val, 'val2');
      const data = {
        name: '',
        id: val,
        customerCode: localstorageGet('customerCode')
      };
      this.$axios.post(Api.templateLibrary.typeList, {
        data,
        pages: 1,
        size: 9999
      }, (res) => {
        console.log(res, 'res');
        if (res.isSuccess) {
          if (res.data.length) {
            this.formData.useScope = res.data[0].useScope;
            this.getFlowAndType(this.formData.useScope);
          }
        }
      });
    },
    // 使用范围变化
    changeUseScope(val) {
      if (val) {
        if (this.formData.typeId) {
          this.formData.typeId = null;
          this.$refs.formData.clearValidate('typeId');
        }
        if (this.formData.groupId) {
          this.formData.groupId = null;
          this.$refs.formData.clearValidate('groupId');
        }
        this.getFlowAndType(val);
      }
    },
    getFlowAndType(val) {
      switch (val) {
        case 'invest':
          this.getFlowGroupList('epc');
          this.formType(val);
          break;
        case 'operation':
          this.getFlowGroupList(val);
          this.formType(val);
          break;
        case 'solar':
          this.getFlowGroupList(val);
          this.formType(val);
          break;
      }
    },
    getSelectCompanyList() { // 查询公司列表
      this.$axios.post(
        Api.taskManage.taskArrange.getCompanyDepartTree,
        {
          data: {
            id: localstorageGet('companyId'),
            flag: 1
          }
        },
        (res) => {
          var arr = [];
          var fn = (list) => {
            list.forEach((item) => {
              if (item.type == 1) {
                arr.push({
                  id: item.id,
                  name: item.name,
                  type: item.type
                });
                if (item.childrenList && item.childrenList.length) {
                  fn(item.childrenList);
                }
              }
            });
          };
          fn(res.data);
          this.relativeList = arr;
        }
      );
    },
    getRelativeList() {
      const data = {
        data: {
          customerCode: this.$store.getters.customerCode,
          companyType: 'RELATED_PARTY_COMPANY' // 公司类型：相关方公司
        },
        pages: 1,
        size: 9999
      };
      this.$axios.post(Api.relatedParties.list, data, (res) => {
        if (res.isSuccess) {
          this.relativeList = res.data?.dataList || [];
        } else {
          this.$message.error(res.message);
        }
      });
    },
    // showSelectBox(val) {
    //   if (val) { // 打开后，并且之前也选择过类型，就给提示
    //     if (this.infoForm.typeId) {
    //       this.$message({
    //         showClose: true,
    //         message: '更改表单类型后，流程设计步骤配置好的节点将自动被清空！',
    //         type: 'error',
    //         duration: '6000'
    //       });
    //     }
    //   }
    // },
    // changeTypeId() {
    //   // Bus.$emit('changeTypeId');
    // },
    // 数据字典大类下的字段列表
    initFieldList(val) {
      this.$axios.post(Api.algorithm.getDicCodeTree, {
        data: {
          dictCode: val
        }
      }, res => {
        if (res.isSuccess) {
          this.fieldTreeList = res.data;
          Bus.$emit('sendFieldTreeList', res.data);
        }
        // else {
        //   this.infoForm.typeId = '';
        //   this.$message.error('该类型为有表单流程类型，请选择其他类型！');
        // }
      });
    },
    getFlowGroupList(useScope) {
      const data = { bizScope: useScope, customerCode: this.$store.getters.customerCode };
      this.$axios.post(Api.frameworkInfo.departmentFramework.flow.getFlowGroupList, {
        data
      }, (res) => {
        if (res.isSuccess && res.data) {
          this.flowList = res.data;
        }
      });
    },
    formType(useScope) {
      this.$axios.post(
        Api.frameworkInfo.departmentFramework.flow.typeList,
        {
          data: {
            name: '',
            useScope: useScope,
            customerCode: this.$store.getters.customerCode
          },
          pages: 1,
          size: 99999
        },
        res => {
          if (res.isSuccess) {
            this.formTypeList = res.data;
            console.log('this.formTypeList', this.formTypeList);
            var formType = this.formTypeList.find(x => x.id == this.formData.typeId);
            if (formType) {
              this.step1AuditWay = formType.auditWay;
              localstorageSet('step1AuditWay', this.step1AuditWay);
              console.log(this.step1AuditWay, 'this.step1AuditWay222');
              this.initFieldList(this.step1AuditWay);
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    }
  }
};
</script>

<style scoped lang="scss">
.info-form-container {

  ::v-deep .el-input,
  .el-textarea {
    width: 400px;
  }
}
</style>
