<!--
 * @Descripttion:创建新流程\基础信息
 * @Author: Calvin
 * @Date: 2021-06-04 11:04:03
-->
<template>
  <div class="info-form-container">
    <el-form ref="infoForm" :model="infoForm" :rules="rules" label-width="200px" label-position="top">
      <el-row :gutter="20">
        <el-col :span="12">
          <el-form-item label="流程名称" prop="flowName">
            <el-input v-model="infoForm.flowName" :disabled="editType == 3" placeholder="请输入流程名称" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="类型" prop="typeId">
            <el-select
              v-model="infoForm.typeId"
              :disabled="editType == 3"
              placeholder="选择类型"
              @change="changeTypeId"
              filterable
            >
              <el-option v-for="item in formTypeList" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>
          </el-form-item>
        </el-col>
      </el-row>
      <el-row :gutter="20">
        <el-col :span="12">
          <el-form-item label="流程分组" prop="groupId">
            <el-select v-model="infoForm.groupId" :disabled="editType == 3" placeholder="请选择流程分组" filterable>
              <el-option v-for="item in flowList" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>
          </el-form-item>
        </el-col>
        <template v-if="isRelative">
          <el-col :span="12">
            <el-form-item :label="$route.query.isGeneralFlow ? '选择公司' : '选择相关方公司'" prop="company">
              <el-select v-model="infoForm.company" :disabled="editType == 3" placeholder="请选择公司" filterable>
                <el-option v-for="item in relativeList" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
        </template>
        <!-- <template v-else>
          <el-col :span="12">
            <el-form-item label="选择公司" prop="company">
              <el-select v-model="infoForm.company" :disabled="editType == 3" placeholder="请选择公司" filterable>
                <el-option v-for="item in companyList" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
          </el-col>
        </template> -->
        <el-col :span="12">
          <el-form-item label="用途说明" prop="remark">
            <el-input
              v-model="infoForm.remark"
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
    var validateGroupId = (rule, value, callback) => {
      if (this.isCurrentPlatform) {
        if (value === '') {
          callback(new Error('请选择分组'));
        } else {
          callback();
        }
      } else {
        callback();
      }
    };
    return {
      isCurrentPlatform: false, // 是否自建流程
      flowList: [],
      formTypeList: [
        // {
        //   id: 1,
        //   name: '费用报销单'
        // },
        // {
        //   id: 2,
        //   name: '部门月度预算'
        // },
        // {
        //   id: 3,
        //   name: '公司预算金额调剂申请单'
        // },
        // {
        //   id: 4,
        //   name: '公司年度预算'
        // },
        // {
        //   id: 5,
        //   name: '追加年度预算'
        // },
        // {
        //   id: 6,
        //   name: '项目立项预算'
        // },
        // {
        //   id: 7,
        //   name: '追加项目预算'
        // }
      ],
      rules: {
        flowName: [
          { required: true, message: '请输入流程名称', trigger: 'blur' }
        ],
        groupId: [
          { required: true, validator: validateGroupId, trigger: 'change' }
        ],
        typeId: [
          { required: true, message: '请选择类型', trigger: 'change' }
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
    // infoFormData: {
    //   handler(val) {
    //     this.infoForm = JSON.parse(JSON.stringify(val));
    //   }
    //   // deep:true
    // },
    'infoForm.typeId': {
      handler(val) {
        let formType = this.formTypeList.find(x => x.id == val)
        if(formType){
          this.step1AuditWay = formType.auditWay;
          localstorageSet('step1AuditWay', this.step1AuditWay);
          this.initFieldList(this.step1AuditWay);
        }
      },
      deep: true
    }
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
    this.getFlowGroupList();
    this.formType();
  },
  methods: {
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
                  type: item.type,
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
    changeTypeId() {
      // Bus.$emit('changeTypeId');
    },
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
    getFlowGroupList() {
      const data = { bizScope: 'epc', customerCode: this.$store.getters.customerCode };
      this.$axios.post(Api.frameworkInfo.departmentFramework.flow.getFlowGroupList, {
        data
      }, (res) => {
        if (res.isSuccess && res.data) {
          this.flowList = res.data;
        }
      });
    },
    formType() {
      this.$axios.post(
        Api.frameworkInfo.departmentFramework.flow.typeList,
        {
          data: {
            name: '',
            useScope: 'invest',
            customerCode: this.$store.getters.customerCode
          },
          pages: 1,
          size: 99999
        },
        res => {
          if (res.isSuccess) {
            this.formTypeList = res.data;
            var formType = this.formTypeList.find(x => x.id == this.infoForm.typeId)
            if (formType) {
              this.step1AuditWay = formType.auditWay;
              localstorageSet('step1AuditWay', this.step1AuditWay);
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
