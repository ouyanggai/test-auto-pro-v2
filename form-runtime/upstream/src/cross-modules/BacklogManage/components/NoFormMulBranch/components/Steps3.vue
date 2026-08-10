
<!--
 * @Descripttion: 流程第三步
 * @Author: zhengzetao
 * @Date: 2021-03-18
-->

<template>
  <div class="flow-main">
    <!-- <div class="flow-btn-box"> -->
    <!-- <el-button
        type="primary"
        @click="handleLast"
        class="btn"
      >上一步</el-button> -->
    <!-- <el-button
        type="primary"
        @click="saveSet"
      >发 布</el-button> -->
    <!-- </div> -->
    <div class="fd-nav-content">
      <section class="dingflow-design">
        <div class="zoom">
          <div :class="'zoom-out' + (nowVal == 50 ? ' disabled' : '')" @click="zoomSize(1)" />
          <span>{{ nowVal }}%</span>
          <div :class="'zoom-in' + (nowVal == 300 ? ' disabled' : '')" @click="zoomSize(2)" />
        </div>
        <div
          id="box-scale"
          class="box-scale"
          :style="'transform: scale(' + nowVal / 100 + '); transform-origin: 50% 0px 0px;'"
        >
          <NodeWrap
            :node-config.sync="nodeConfig"
            :json-data.sync="templateData"
            :fields.sync="fields"
            :company-person-list.sync="companyPersonList"
            :field-tree-list.sync="fieldTreeList"
            :step1AuditWay.sync="step1AuditWay"
            :edit-type.sync="editType"
            :props-role-list.sync="propsRoleList"
            :attr-list="attrList"
          />
          <!-- :flowPermission.sync="flowPermission"
            :isTried.sync="isTried"
            :roleList.sync="roleList"
            :personList.sync="personList" -->
          <div class="end-node">
            <div class="end-node-circle" />
            <div class="end-node-text">流程结束</div>
          </div>
        </div>
      </section>
    </div>

  </div>
</template>

<script>
import Api from '@/api';
// import { getFlowWork, getFlowData } from '@/api/user';
import Bus from '@/bus';
import { mapGetters } from 'vuex';
import { localstorageGet } from '@/utils/auth';
import NodeWrap from './Workflow/nodeWrap';

export default {
  name: 'Steps3',
  components: { NodeWrap },
  // props: {
  //   flowNodeTemplateData: {
  //     type: Object,
  //     default: () => {
  //       return {};
  //     }
  //   },
  //   id: {
  //     type: String,
  //     default: ''
  //   },
  //   fromId: {
  //     type: String,
  //     default: ''
  //   }
  // },
  props: {
    // 流程节点
    // flowNodes: {
    //   type: Array,
    //   default: () => []
    // },
    propsRoleList: {
      type: Array,
      default: () => []
    },
    attrList: {
      type: Array,
      default: () => []
    },
    flowNodes: {
      type: Object,
      default: () => {}
    },
    step1AuditWay: {
      type: String
    },
    fields: {
      type: Array,
      default: () => []
    },
    fieldTreeList: {
      type: Array,
      default: () => []
    },
    companyPersonList: {
      type: Array,
      default: () => []
    },
    formEditData: {
      type: Object
    },
    templateData: {
      type: Object
    },
    editType: {
      type: [String, Number],
      default: 1
    },
    isNoFormFlow: {
      type: Boolean,
      default: false
    },
    isInputFlow:{
      type:String,
      default:''
    },
  },
  data() {
    return {
      infoForm: {},
      isTried: false,
      tipList: [],
      // roleList: [],
      personList: [],
      tipVisible: false,
      nowVal: 100,
      processConfig: {},
      nodeConfig: {
        nodeName: '发起人',
        error: false,
        type: 'start',
        flowNodeAuditConfig: {
          auditType: 'assign',
          countersignNum: 1,
          type: "scramble",
          nodeAuditScopeList:[],
          flowNodeDetailConfigList: [
            {
              auditDetailType: "personnel",
              bizId: localstorageGet('userId'),
              id: localstorageGet('userId'),
              name: localstorageGet('userName')
            }
          ]
        },
        flowNodeFieldPowerTemplateList: [],
        childFlowNodeTemplate: null
      },
      // nodeConfig: {
      //   nodeName: '发起人',
      //   error: false,
      //   type: 'start',
      //   flowNodeAuditConfig: {
      //     auditType: null,
      //     flowNodeDetailConfigList: []
      //   },
      //   flowNodeFieldPowerTemplateList: [],
      //   childFlowNodeTemplate: null
      // },
      flowPermission: [],
      dialogVisible: true,
      // fields: [],
      jsonData: {},
      jsonDataDefault: {},
      verifyFlag: []
    };
  },
  computed: {
    ...mapGetters([
      'stationId'
    ])
  },
  watch: {
    flowNodes: { // 深度监听，可监听到对象、数组的变化
      handler(val) {
        console.log('回显编辑节点',this.editType)
        console.log('回显编辑节点2',this.isInputFlow)
        if (this.editType != 1 || this.isInputFlow) {
          this.nodeConfig = val;
        }
      },
      deep: true
    },
    // flowNodeTemplateData: { // 深度监听，可监听到对象、数组的变化
    //   handler(val) {
    //     if (this.id) {
    //       this.nodeConfig = val;
    //     }
    //   }
    //   // deep: true
    // }
  },
  created() {
    console.log('111111111-this.flowNodes',this.flowNodes);
    if (Object.keys(this.flowNodes).length) {
      this.nodeConfig = this.flowNodes;
    }

    Bus.$on('changeTypeId', (data) => {
      this.nodeConfig = {
        nodeName: '发起人',
        error: false,
        type: 'start',
        flowNodeAuditConfig: {
          auditType: 'assign',
          countersignNum: 1,
          type: "scramble",
          flowNodeDetailConfigList: [
            {
              auditDetailType: "personnel",
              bizId: localstorageGet('userId'),
              id: localstorageGet('userId'),
              name: localstorageGet('userName')
            }
          ]
        },
        // flowNodeAuditConfig: {
        //   auditType: null,
        //   flowNodeDetailConfigList: []
        // },
        flowNodeFieldPowerTemplateList: [],
        childFlowNodeTemplate: null
      };
    });
    // Bus.$on('sendFormJson', (data) => {
    //   this.jsonData = JSON.parse(JSON.stringify(data));
    // });
    // Bus.$on('sendFormJsonDefault', (data) => {
    //   this.jsonDataDefault = data;
    // });
    // Bus.$on('sendFields', (data) => {
    //   this.fields = [];
    //   //  JSON.parse(JSON.stringify(data));
    //   this.fields = data;
    // });
    // this.getFlowRole();
    // this.getFlowPerson();
  },
  mounted() { },
  methods: {
    handleLast() {
      this.$emit('updateActive', 1);
      // window.sessionStorage.setItem('sendFieldsArr', JSON.stringify(this.fields));
    },
    // 创建流程
    createCustomFlow(flowNode) {
      // this.$axios.post(Api.flow.createCustomFlowTemplate, {
      //   data: flowNode,
      //   formTemplateBizRelevance: { stationId: this.stationId }
      // }, (res) => {
      //   if (res.isSuccess) {
      //     this.$router.push({ name: 'DepartmentFrameworkCustomFlowNew' });
      //   } else {
      //     this.$message.error(res.message);
      //   }
      // });

      this.$axios.post(
        Api.frameworkInfo.departmentFramework.flow.saveCustomFlowTemplate,
        {
          data: flowNode,
          formTemplateBizRelevance: {
            projectId: localstorageGet('projectId')
          }
        },
        res => {
          // this.loading = false;
          if (res.isSuccess) {
            this.$message.success('保存成功');
            this.$router.push({ name: 'DepartmentFrameworkCustomFlowNew' });
            // this.goback();
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 编辑流程 editCustomFlowTemplate
    editCustomFlowTemplate(flowNode) {
      this.$axios.post(Api.flow.editCustomFlowTemplate, {
        data: flowNode,
        formTemplateBizRelevance: { stationId: this.stationId }
      }, (res) => {
        if (res.isSuccess) {
          this.$router.push({ name: 'formLibrary' });
        } else {
          this.$message.error(res.message);
        }
      });
    },
    verifyNodeConfigData(data) {
      const obj = data;
      const { flowNodeAuditConfig, childFlowNodeTemplate, type } = obj;
      if (type != 'condition') {
        if (flowNodeAuditConfig) {
          if (!flowNodeAuditConfig.auditType) {
            // this.$message.error(`存在流程节点没有选择审批类型`); // 什么是审批类型
            // this.verifyFlag.push(1);
          } else if (flowNodeAuditConfig.auditType == 'role' && !flowNodeAuditConfig.flowNodeDetailConfigList.length) {
            this.$message.error(`审批类型为选择角色时请添加角色`);
            this.verifyFlag.push(1);
          } else if (flowNodeAuditConfig.auditType == 'assign' && !flowNodeAuditConfig.flowNodeDetailConfigList.length) {
            this.$message.error(`审批类型为选择人员时请添加人员`);
            this.verifyFlag.push(1);
          }
        }
      } else {
        const { conditionNodes } = obj;
        let conditionFlag = false;
        conditionNodes.map(item => {
          if (item.childFlowNodeTemplate) {
            this.verifyNodeConfigData(item.childFlowNodeTemplate);
          } else {
            conditionFlag = true;
          }
        });
        if (conditionFlag) {
          this.$message.error(`每一条分支必须存在流程节点`);
          this.verifyFlag.push(1);
        }
      }
      if (childFlowNodeTemplate) {
        this.verifyNodeConfigData(childFlowNodeTemplate);
      }
    },
    verifyData() {
      const nodeConfig = this.nodeConfig;
      let flag = false;
      if (!nodeConfig.childFlowNodeTemplate) {
        flag = true;
        this.$message.error(`流程设计至少两个审批节点`);
      } else {
        this.verifyNodeConfigData(nodeConfig);
      }
      return flag;
    },
    // saveSet() {
    //   this.verifyFlag = [];
    //   this.verifyFlag.length = 0;
    //   if (this.verifyData()) {
    //     return;
    //   }
    //   if (this.verifyFlag.length) {
    //     return;
    //   }
    //   this.isTried = false;
    //   this.tipList = [];
    //   this.processConfig.flowPermission = this.flowPermission;
    //   this.generateModle(this.jsonData.list || []);
    //   const flowNode = {
    //     flowName: this.infoForm.name + '流程',
    //     remark: this.infoForm.remark,
    //     formTemplateVo: {
    //       name: this.infoForm.name,
    //       templateData: this.jsonDataDefault, // JSON.stringify(this.jsonData)
    //       fieldsTemplateList: this.fields
    //     },
    //     flowNodeTemplate: this.nodeConfig
    //   };
    //   console.log('发布', flowNode);
    //   const isTemplate = this.$route.query.isTemplate;
    //   if (this.id) {
    //     if (isTemplate == 1) {
    //       this.createCustomFlow(flowNode);
    //     } else {
    //       flowNode.id = this.id;
    //       flowNode.formTemplateVo.id = this.fromId;
    //       this.editCustomFlowTemplate(flowNode);
    //     }
    //   } else {
    //     this.createCustomFlow(flowNode);
    //   }
    // },

    // 表单数据处理方法 递归，将表单模板input 改成CheckBox
    generateModle(genList) {
      genList.map((item, key) => {
        if (item.type == 'grid') {
          item.columns.map(val => {
            this.generateModle(val.list);
          });
        } else if (item.type == 'report') {
          item.rows.map(val => {
            val.columns.map(i => {
              this.generateModle(i.list);
            });
          });
        } else {
          if (item.model) {
            const obj =
            {
              type: 'checkbox',
              icon: 'icon-check-box',
              options: {
                inline: false,
                defaultValue: [
                  '编辑'
                ],
                showLabel: true,
                options: [
                  {
                    value: '编辑',
                    label: ''
                  }
                ],
                required: false,
                requiredMessage: '',
                width: '',
                remote: false,
                remoteType: 'datasource',
                remoteOption: item.remoteOption,
                remoteOptions: [],
                props: {
                  value: 'value',
                  label: 'label'
                },
                remoteFunc: item.remoteFunc,
                customClass: '',
                labelWidth: 10,
                isLabelWidth: true,
                hidden: false,
                dataBind: true,
                disabled: this.disabledCellType == 1,
                hideLabel: false
              },
              events: {
                onChange: ''
              },
              name: '',
              key: item.key,
              model: item.model,
              rules: []
            };
            this.$set(genList, key, obj);
            // 初始话选中的字段
            // const fieldsObj = {
            //   name: item.model,
            //   englishName: item.model,
            //   fieldType: 'stringType',
            //   length: 999,
            //   defaultValue: '',
            //   remark: '备注',
            //   standby: '',
            //   fieldStatus: 'enable',
            //   valueOrigin: 'fromUser'
            // };
            // this.fields.push(fieldsObj);
          }
        }
      });
    },
    // getFlowRole() {
    //   const data = {};
    //   this.$axios.post(Api.flowRole.list, {
    //     data
    //   }, (res) => {
    //     if (res.isSuccess) {
    //       this.roleList = res.data || [];
    //     }
    //   });
    // },
    // getFlowPerson() {
    //   const data = {
    //     customerCode: this.$store.state.user.customerCode
    //   };
    //   this.$axios.post(Api.flowPerson.findAllByPlatformCode, {
    //     data
    //   }, (res) => {
    //     if (res.isSuccess) {
    //       this.personList = res.data || [];
    //     }
    //   });
    // },
    zoomSize(type) {
      if (type == 1) {
        if (this.nowVal == 50) {
          return;
        }
        this.nowVal -= 10;
      } else {
        if (this.nowVal == 300) {
          return;
        }
        this.nowVal += 10;
      }
    }
  }
};
</script>
<style lang='scss' scoped>
// @import "~@/assets/styles/override-element-ui.scss";
@import "~@/assets/styles/workflow.scss";

.flow-main {
  padding-top: 60px;
  background-color: #fff;

  >div {
    padding: 0 !important;
  }
}

.flow-btn-box {
  position: fixed;
  top: 230px;
  right: 40px;
  color: #333;
  z-index: 10;
}

.dialog-content {
  padding: 10px 10px;
  margin-left: 10px;
}

.fd-nav-content {
  top: 54px;
  left: 120px;
  height: calc(100% - 136px);
  // top: 226px;
  // left: 120px;
  // height: calc(100% - 300px);
}

.dingflow-design {
  background: #fff;
  // position: relative;
}

.dingflow-design .zoom {
  right: 30px;
}

.dingflow-design .branch-box .col-box {
  background: #fff;
  // background: #f5f5f7;
}

.bottom-left-cover-line,
.bottom-right-cover-line,
.top-left-cover-line,
.top-right-cover-line {
  background-color: #fff;
}
</style>

