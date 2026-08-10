<!--
 * @Descripttion:组织架构 / 架构信息 / 项目部架构 / 自定义流程
 * @Author: Calvin
 * @Date: 2021-06-07 16:23:12
-->
<template>
  <!-- v-loading='loading' -->
  <div class="flow-container">
    <!-- <template>
      <el-button icon="el-icon-back" @click="goback" style="position:absolute;z-index:2">返 回</el-button>
      <el-steps class="steps-container" :active="stepsActive" align-center>
        <el-step title="基础信息" description="" />
        <template v-if="isNoFormFlow">
          <el-step title="表单设计" description="" />
        </template>
        <el-step title="流程设计" description="" />
      </el-steps>
    </template> -->
    <div class="flow-content">
      <!-- <Steps1 v-show="stepsActive == 1" ref="steps1" :edit-type="editType" :info-form="formDetailData.infoForm"
        :company-list="companyList" :formModelNo="formModelNo"/>
      <template v-if="isNoFormFlow">
        <Steps2 v-show="stepsActive == 2" ref="steps2" :edit-type="editType" :json-data="formDetailData.jsonData"
          :form="formDetailData.form" :isInputFlow="isInputFlow"/>
      </template> -->
      <Steps3 ref="steps3" :template-data="templateData" :form-edit-data="formEditData"
        :flow-nodes="formDetailData.flowNodes" :edit-type="editType"
        :is-no-form-flow="isNoFormFlow" :fields="fields" :company-person-list="companyPersonList"
        @deleteNode="deleteNode" :isInputFlow="isInputFlow" :differentList="differentList" :props-role-list.sync="roleList" :attr-list="attrList"
        :flowNodeProxyId="flowNodeProxyId"/>
    </div>
    <div class="btn-container">
      <el-button type="primary" @click="saveCustomFlowTemplate">保存</el-button>
      <!-- <el-button v-show="stepsActive != 1" @click="prev">上一步</el-button>
      <el-button v-show="stepsActive != (isNoFormFlow ? 3 : 2)" type="primary" @click="next">下一步</el-button>
      <el-button v-show="stepsActive == (isNoFormFlow ? 3 : 2) && editType != 3" type="primary"
        @click="saveCustomFlowTemplate">提交</el-button> -->
    </div>
    <!-- 快速配置流程回调 -->
    <!-- <el-dialog v-if="callBackDialog" title="配置流程回调" :visible="callBackDialog" width="500px" @close="handleClose"
      :close-on-click-modal="false">
      <el-row>
        <el-col :span="8">触发配置名称</el-col>
        <el-col :span="16">
          <el-input v-model="flowBackConfigName" style="width: 180px;" />
        </el-col>
      </el-row>
      <el-row style="margin-top:20px;">
        <el-col :span="8">回调选择</el-col>
        <el-col :span="16">
          <el-select v-model="chooseFlowBack" style="width: 180px;" filterable>
            <el-option v-for="item in flowBackList" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-col>
      </el-row>
      <span slot="footer" class="dialog-footer">
        <el-button @click="handleClose">取 消</el-button>
        <el-button type="primary" @click="confirmFlowConfig">确 定</el-button>
      </span>
    </el-dialog> -->
  </div>
</template>

<script>
import Api from '@/api';
import { arrayToTree, treeToArray, deepClone,uuid } from '@/utils';
import Steps1 from './components/Steps1';
import Steps2 from './components/Steps2';
import Steps3 from './components/Steps3';
import { localstorageGet, localstorageSet, localstorageRemove } from '@/utils/auth';
import mixin from '../commonFlow/mixin.js';

export default {
  name: '',
  components: {
    Steps1,
    Steps2,
    Steps3
  },
  props:['outFlowNodeTemplate','flowNodeProxyId','flowInstanceId'],
  data() {
    return {
      loading: false,
      isGoback: false,
      code: '',
      stepsActive: 1,
      editType: 2,
      // editType: 1,
      isCopyFlow: false,
      projectId: '',
      templateId: '',
      editDataId: '', // 编辑流程表单的id
      templateData: {
        config: {},
        list: []
      },
      cacheTemplateData: {}, // 缓存表单字段
      formEditData: {}, // 表单字段
      formDetailData: {
        infoForm: {
          flowName: '',
          groupId: '',
          remark: ''
        },
        flowNodes: {},
        form: {
          name: '',
          jsonData: {}
        }
      },
      fields: [], // 字段列表
      verifyFlag: [],
      oldFieldsList: [],
      companyPersonList: [],
      companyList: [],
      company: '',
      // 回调配置
      callBackDialog: false,
      flowBackList: [],
      chooseFlowBack: '',
      flowBackConfigName: '',
      isInputFlow:'',
      differentList:[],
      //属性选择
      attrList:[],
      formModelNo:''
    };
  },
  mixins:[mixin],
  computed: {
    isNoFormFlow: function () {
      return this.$route.query.processTemplateIdx != 3;
    }
  },
  beforeRouteEnter(to, from, next) {
    const fromUrl = from.fullPath;
    to.meta.activeMenu = fromUrl;
    next();
  },
  watch: {
    stepsActive(val) {
      if (val > 1) {
        if (this.$route.query?.relative) localstorageSet('formDetailCompany', this.formDetailData.infoForm.company);
        else {
          localstorageRemove('formDetailCompany');
        }
      }
    }
  },
  created() {
    this.getRoleList(); // 用于审批人范围选择回显
    this.getAttrList(); // 获取属性列表

    // editType：1：新增；2：编辑；3：查看
    this.projectId = localstorageGet('projectId');
    this.templateId = null;
    console.log(1111,this.type)
    this.editType = this.type == 'add' ? 1 : 2;

    this.getCompanyPersonList();
    if (this.editType == 2) { // 编辑或查看
      console.log('编辑流程')
      // this.code = this.$route.query.code;

      this.formDetailData = {
        // 第三步详情
        flowNodes: this.outFlowNodeTemplate.flowNodeTemplate
        // flowNodes: this.outFlowNodeTemplate
      };
      console.log('formDetailData',this.formDetailData)
      // this.getFormDetail();
    } else {
      console.log('发起流程')
      // this.getCompanyPersonList();
      // this.getCompanyList(); // 暂时不用选择公司的逻辑
    }
    
    // this.projectId = localstorageGet('projectId');
    // this.templateId = this.$route.query.templateId;
    // this.editType = this.$route.query.editType;
    // this.isCopyFlow = this.$route.query.isCopyFlow;
    // this.isInputFlow = this.$route.query.isInputFlow
    // if (this.isInputFlow) {
    //   //导入流程数据
    //   let jsonStringData = localstorageGet('inputJSONFile')
    //   if (jsonStringData) {
    //     let jsonParseData = JSON.parse(jsonStringData)
    //     setTimeout(()=>{
    //       this.initInputFlowData(jsonParseData)
    //       localstorageRemove('inputJSONFile')
    //     })
    //   }
    //   this.getCompanyPersonList();
    // } else {
    //   if (this.templateId) { // 编辑或查看
    //     this.code = this.$route.query.code;
    //     this.getFormDetail();
    //   } else {
    //     this.getCompanyPersonList();
    //     this.getCompanyList(); // 暂时不用选择公司的逻辑
    //   }
    // }

  },
  mounted() {

  },
  destroyed() {
    // 解决Vue $on能拿到数据但是无法更新data数据
    if (this.isGoback) {
      this.$bus.$emit('activeName', 'examProcess');
    }
    // 在平台离开部门页面需要清空项目id，部门间页面跳转不需要清空；另外需要区分平台和项目
    if (!!this.$store.state.app.toRouteType && !this.$store.state.app.isDepartmentFramework) {
      this.$store.commit('user/SAVE_PROJECT_ID', '');
    }
  },
  methods: {
    // 获取属性
    getAttrList() {
      this.$axios.post(
        '/web/user/api/expandAttr/list',
        {
          data: {
            name:'',
            expandAttrType:null,
            enableType:"enable"
          },
          // pagination: true,
          // current: this.pagination.pages,
          // size: this.pagination.size
        },
        res => {
          if (res.isSuccess) {
            this.attrList = res.data || [];
            // this.pagination.total = res.total;
          }
        }
      );
    },
    // initInputFlowData(data) {
    //   const formTemplateData = data.formTemplateList[0];
    //   // return
    //   // this.editDataId = formTemplateData.id;
    //   // const flowNodeTemplate = treeToArray(data.flowNodeTemplate);
    //   // console.log('flowNodeTemplate', flowNodeTemplate);
    //   let templateData = data.flowNodeTemplate
    //   // let flowNodeTemplate = {
    //   //   nodeName : templateData.nodeName,
    //   //   error:false,
    //   //   type:templateData.type,
    //   // }
    //   //数据清洗 去掉id 去掉公司 去掉人员
    //   var fun = (templateData)=>{
    //     if(templateData.id)delete templateData.id
    //     if(templateData?.flowNodeAuditConfig)delete templateData?.flowNodeAuditConfig.id
    //     if(templateData?.flowNodeAuditConfig?.flowNodeDetailConfigList)templateData.flowNodeAuditConfig.flowNodeDetailConfigList = []
    //     if(templateData?.flowNodeFieldPowerTemplateList){
    //       let flowNodeFieldPowerTemplateList = templateData.flowNodeFieldPowerTemplateList
    //       let tmp = []
    //       flowNodeFieldPowerTemplateList.forEach(el=>{
    //         tmp.push({
    //           fieldPower: el.fieldPower,
    //           formFieldTemplateEnglishName: el.formFieldTemplateEnglishName
    //         })
    //       })
    //       templateData.flowNodeFieldPowerTemplateList = tmp
    //     }
    //     //分支
    //     if(templateData.type == 'condition'){
    //       let conditionNodes = templateData.conditionNodes || []
    //       conditionNodes.forEach(el=>{
    //         if(el.id)delete el.id
    //         if(el.childFlowNodeTemplate){
    //           fun(el.childFlowNodeTemplate)
    //         }
    //       })
    //     }
    //     if(templateData.childFlowNodeTemplate){
    //       fun(templateData.childFlowNodeTemplate)
    //     }

    //     // flowNodeTemplate.childFlowNodeTemplate = {
    //     //   nodeName : templateData.nodeName,
    //     //   error:false,
    //     //   type:templateData.type,
    //     // }
    //     // if(templateData.type == 'condition'){
    //     //   if(templateData.branchExecuteType){
    //     //     flowNodeTemplate.branchExecuteType = templateData.branchExecuteType
    //     //   }
    //     //   let conditionNodes = templateData.conditionNodes
    //     //   conditionNodes.forEach(el=>{

    //     //   })
    //     // }else{
    //     //   if(templateData.childFlowNodeTemplate)fun(templateData.childFlowNodeTemplate)
    //     // }

    //   }
    //   fun(templateData)
    //   // 获取表单初始的字段
    //   this.getAllFieldsList(JSON.parse(formTemplateData.templateData).list);

    //   this.formDetailData = {
    //     // 第一步详情
    //     infoForm: {
    //       flowName: data.flowName,
    //       groupId: data.groupId,
    //       remark: data.remark,
    //       typeId: data.typeId,
    //       company: data.formTemplateBizRelevanceVoList && data.formTemplateBizRelevanceVoList.find(x => x.otherBiz == 'company') ? data.formTemplateBizRelevanceVoList.find(x => x.otherBiz == 'company').otherBizId : ''
    //     },
    //     // 第二步详情
    //     form: {
    //       name: formTemplateData.name,
    //       jsonData: JSON.parse(formTemplateData.templateData)
    //     },
    //     // 第三步详情
    //     flowNodes: templateData
    //   };
    //   // if (this.$route.query?.relative) {
    //   //   this.company = this.formDetailData.infoForm.company;
    //   // }
    // },
    //获取回调的列表
    getCallBackList() {
      return new Promise((resolve, reject) => {
        const data = {
          customerCode: this.$store.getters.customerCode
        };
        this.$axios.post(Api.flowCallBackConfig.getCallBackList, { data }, res => {
          resolve(res);
        });
      });
    },
    confirmFlowConfig() {
      if (!this.flowBackConfigName || !this.chooseFlowBack) {
        return this.$message.error('请填写配置信息');
      }
      // 保存配置名称
      const data = {
        bizType: 'FLOW',
        isAsync: false,
        isOpen: true,
        model: 'CUSTOM_PROCESSING',
        name: this.flowBackConfigName,
        retry: 1
      };
      this.saveTriggerConfig(data).then(res => {
        if (res.isSuccess) {
          const id = res.data.id;
          this.bindingConfig(id).then(res => {
            if (res.isSuccess) {
              this.bindFlowandBack(this.flowBackConfigName, this.curruntflowCode, id).then(res => {
                if (res.isSuccess) {
                  this.$message.success('操作成功');
                  this.handleClose();
                } else {
                  this.$message.error(res.message);
                }
              });
            } else {
              this.$message.error(res.message);
            }
          });
        } else {
          this.$message.error(res.message);
        }
      });
    },
    bindFlowandBack(flowName, flowCode, requestTriggerConfigId) {
      const data = {
        bizName: flowName + '流程回调',
        executeStatus: 'pass',
        executeType: 'unconditional',
        flowCode,
        requestTriggerConfigId,
        type: 'all'
      };
      return new Promise((resolve, reject) => {
        this.$axios.post(Api.flowCallBackConfig.saveFlowConfigRelation, { data }, res => {
          resolve(res);
        });
      });
    },
    bindingConfig(id) {
      const data = {
        data: {},
        requestTriggerLinkList: [{
          requestConfigId: this.chooseFlowBack,
          requestTriggerConfigId: id
        }
        ]
      };
      return new Promise((resolve, reject) => {
        this.$axios.post(Api.flowCallBackConfig.configRelateCallback, data, res => {
          resolve(res);
        });
      });
    },
    saveTriggerConfig(data) {
      return new Promise((resolve, reject) => {
        this.$axios.post(Api.flowCallBackConfig.saveTriggerConfig, { data }, res => {
          resolve(res);
        });
      });
    },
    handleClose() {
      this.callBackDialog = false
      this.goback();
    },
    getCompanyList() {
      this.$axios.post(
        Api.frameworkInfo.getParentCompanyList,
        {
          data: {
            id: this.company || this.$store.state.user.companyId
          }
        },
        res => {
          if (res.isSuccess) {
            this.companyList = res.data;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 获取流程详情
    // getFormDetail() {
    //   console.log('获取流程详情getFormDetail')
    //   const data = {
    //     id: this.templateId,
    //     code: this.code
    //   };
    //   this.$axios.post(
    //     Api.frameworkInfo.departmentFramework.flow.getFlowTemplateDetail,
    //     {
    //       data
    //     },
    //     res => {
    //       if (res.isSuccess) {
    //         const data = res.data;
    //         // this.formTemplateBizRelevanceVoList = data.formTemplateBizRelevanceVoList.map(item=>{
    //         //   return {
    //         //     otherBiz:item.otherBiz,
    //         //     otherBizId:item.otherBizId
    //         //   }
    //         // }) || []
    //         // let find = this.formTemplateBizRelevanceVoList.find(item=>item.otherBiz == 'formModelNo')
    //         // console.log('find',find)
    //         // if(find){
    //         //   this.formModelNo = find.otherBizId
    //         // }
    //         // const formTemplateData = data.formTemplateList[0];
    //         // console.log('获取流程详情getFormDetail',formTemplateData.id)
    //         // console.log('editType',this.editType)
    //         // console.log('isCopyFlow',this.isCopyFlow)
    //         // this.editDataId = formTemplateData.id;
    //         const flowNodeTemplate = data.flowNodeTemplate;
    //         // 获取表单初始的字段
    //         // this.getAllFieldsList(JSON.parse(formTemplateData.templateData).list);

    //         this.formDetailData = {
    //           // 第一步详情
    //           // infoForm: {
    //           //   flowName: data.flowName,
    //           //   groupId: data.groupId,
    //           //   remark: data.remark,
    //           //   typeId: data.typeId,
    //           //   company: data.formTemplateBizRelevanceVoList && data.formTemplateBizRelevanceVoList.find(x => x.otherBiz == 'company') ? data.formTemplateBizRelevanceVoList.find(x => x.otherBiz == 'company').otherBizId : ''
    //           // },
    //           // // 第二步详情
    //           // form: {
    //           //   name: formTemplateData.name,
    //           //   jsonData: JSON.parse(formTemplateData.templateData)
    //           // },
    //           // 第三步详情
    //           flowNodes: flowNodeTemplate
    //         };
    //         // if (this.$route.query?.relative) {
    //         //   this.company = this.formDetailData.infoForm.company;
    //         // }
    //         this.getCompanyPersonList();
    //         // this.getCompanyList(); // 暂时不用选择公司的逻辑 --->兼容一下费用预算的旧版本，需要选一下公司
    //       } else {
    //         this.$message.error(res.message);
    //       }
    //     }
    //   );
    // },
    getCompanyList() {
      this.$axios.post(
        Api.frameworkInfo.getParentCompanyList,
        {
          data: {
            id: this.company || this.$store.state.user.companyId
          }
        },
        res => {
          if (res.isSuccess) {
            this.companyList = res.data;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 用于人员回显
    getCompanyPersonList() {
      const param = {
        data: {
          companyId: this.company || this.$store.state.user.companyId
        },
        pagination: true,
        pages: 1,
        size: 1000
      };
      this.$axios.post(
        Api.customFlow.findByCompanyIdUserList,
        param,
        res => {
          this.loading = false;
          if (res.isSuccess) {
            this.companyPersonList = res.data.dataList;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    goback() {
      console.log('返回')
      console.log('this.$route.query', this.$route.query)
      this.isGoback = true;
      if (this.$route.query?.relative) {
        if (this.$route.query?.isGeneralFlow) {
          this.$router.push({
            path: '/flowLibrary/generalFlow'
          });
        } else {
          this.$router.push({
            path: '/flowLibrary/relative'
          });
        }
      } else {
        // this.$router.push({
        //   path: '/flowLibrary/flowTypeLibrary'
        // });
        // this.$router.push({
        //   path: '/flowLibrary/index'
        // });
        if (this.$route.query.flowTypePage) {
          this.$router.go(-1)
        } else {
          this.$router.push({
            path: '/flowLibrary/index'
          });
        }
      }
    },
    deleteNode(i) {
      if (this.editType != 1) {
        return;
      }
      const arr = deepClone(this.formDetailData.flowNodes);
      if (arr.length == 1) {
        this.$message.error('流程至少保留一项');
        return;
      }
      arr.splice(i, 1);
      arr.map((item, index) => {
        item.sortId = index + 1;
      });
      this.formDetailData.flowNodes = arr;
    },
    // 上一步
    prev() {
      this.stepsActive--;
      if (this.stepsActive == 2) {
        this.oldFieldsList = this.fields.map(x => x.englishName);
      }
    },
    // 下一步
    // next() {
    //   // 第一步
    //   if (this.stepsActive == 1) {
    //     const infoForm = this.$refs.steps1.$refs.infoForm;
    //     let flag = false;
    //     infoForm.validate((valid) => {
    //       if (valid) {
    //         flag = true;
    //       } else {
    //         flag = false;
    //       }
    //     });
    //     if (!flag) {
    //       return;
    //     }
    //     // 第二步
    //   } else if (this.stepsActive == 2 & this.isNoFormFlow) {
    //     if (!this.$refs.steps2.form.name) {
    //       this.$message.error('请输入表单名称');
    //       return;
    //     }
    //     this.fields = [];
    //     // 获取表单模板数据
    //     if (this.editType == 3) {
    //       this.templateData = JSON.parse(JSON.stringify(this.formDetailData.form.jsonData));
    //     } else {
    //       const makingformJson = this.$refs.steps2.$refs.makingform.getJSON();
    //       //兼容一下，旧版本这里会取得一个对象，现在是字符串
    //       if (typeof (makingformJson) == 'string') {
    //         this.templateData = JSON.parse(makingformJson);
    //         this.cacheTemplateData = JSON.parse(makingformJson);
    //       } else {
    //         this.templateData = JSON.parse(JSON.stringify(makingformJson));
    //         this.cacheTemplateData = JSON.parse(JSON.stringify(makingformJson));
    //       }
    //     }
    //     if (!this.templateData.list.length) {
    //       this.$message.error('未填写表单模板');
    //       return;
    //     }
    //     // 添加固定的上传按钮
    //     this.genAttachButton(this.templateData);
    //     // 添加结束
    //     // 处理表单字段
    //     this.generateModle(this.templateData.list);
    //     // console.log('this.fields',this.fields)
    //     // console.log('this.getFieldsList(this.templateData.list)', this.getFieldsList(this.templateData.list));
    //     // 获取表单字段数据
    //     this.fields.map((item, key) => {
    //       this.$set(this.formEditData, item.englishName, false);
    //     });

    //     // 如果修改表单字段，接口返回的流程数据，不存在的表单字段要删除(新增和编辑都需要)
    //     const nodeConfig = this.$refs.steps3.nodeConfig;
    //     this.newFieldsList = this.fields.map(x => x.englishName);
    //     console.log('nodeConfig', nodeConfig)
    //     this.getAllNode(nodeConfig);
    //   }
    //   if (this.stepsActive++ > 2) this.stepsActive = 1;
    // },

    // 获取数组不同元素
    getArrDifference(arr1, arr2) {
      var arr = [];
      var bool = false;
      for (var i = 0; i < arr1.length; i++) {
        for (var j = 0; j < arr2.length; j++) {
          // 进行优化遇到相同直接跳出循环 同时支持对象比对
          if (JSON.stringify(arr1[i]) === JSON.stringify(arr2[j])) {
            bool = false;
            break;
          } else {
            bool = i;
          }
        }
        if (bool !== false) arr.push(arr1[bool]);
      }
      return arr;
    },
    // 遍历流程节点删除不存在的表单字段
    getAllNode(obj) {
      const { flowNodeFieldPowerTemplateList, childFlowNodeTemplate, type } = obj;
      const differentList = this.getArrDifference(this.oldFieldsList, this.newFieldsList);
      // console.log('differentList',differentList)
      this.differentList = differentList;
      if (type == 'condition') {
        obj.conditionNodes.map(item => {
          if (differentList.length) { // 两种做法：第一种只要有不同就全部重新选择；第二种判断有什么不同删除对应不同
            // 第一种：
            // this.$set(item, 'conditionList', []);

            // 第二种：
            differentList.forEach((x) => {
              item.conditionList.forEach((y, i) => {
                let newF = '';
                if (y.fieldaName.endsWith('__condition')) {
                  newF = y.fieldaName.split('__condition')[0];
                } else {
                  newF = y.fieldaName;
                }
                if (x == newF) {
                  item.conditionList.splice(i, 1);
                }
                // if (x == y.fieldaName) {
                //   item.conditionList.splice(i, 1);
                // }
              });
              // console.log('getAllNode-conditionList',item.conditionList)
              // 更新表单字段或者更换组件，导致的分支删除，有可能分支是最后一个，后端要求conditionType为null
              if (item.conditionList.length == 1) {
                item.conditionList[0].conditionType = null;
              }
            });
          }
          if (item.childFlowNodeTemplate) {
            this.getAllNode(item.childFlowNodeTemplate);
          }
        });
      } else {
        if (differentList.length) {
          // 第一种：
          // this.$set(obj, 'flowNodeFieldPowerTemplateList', []);

          // 第二种：
          differentList.forEach((x) => {
            flowNodeFieldPowerTemplateList && flowNodeFieldPowerTemplateList.forEach((y, i) => {
              if (x == y.formFieldTemplateEnglishName) {
                flowNodeFieldPowerTemplateList.splice(i, 1);
              }
            });
          });
        }
      }
      if (childFlowNodeTemplate) {
        this.getAllNode(childFlowNodeTemplate);
      }
    },
    // 递归，将表单模板input 改成CheckBox
    generateModle(genList) {
      // console.log('===generateModle===')
      genList.map((item, key) => {
        if (item.model.endsWith('_showIfPermission')) {
          item.options.hidden = false;
        }

        // 子表单弄不了权限配置，因为有的子表单本身是隐藏的
        // item.type == 'subform' // 子表单+ 容器（支持嵌套复杂布局字段）
        // if (item.type == 'table') { // 子表单容器（不支持布局嵌套）
        //   console.log('item.tableColumns',item.tableColumns)
        //   this.generateModle(item.tableColumns);
        // }
        if (item.type == 'grid') { // 栅格布局
          item.columns.map(val => {
            this.generateModle(val.list);
          });
        } else if (item.type == 'report') { // 表格布局
          item.rows.map(val => {
            val.columns.map(i => {
              this.generateModle(i.list);
            });
          });
        } else if (item.type == 'inline') { // 行内布局
          this.generateModle(item.list);
        } else {
          // if (item.model) {
          //   const obj =
          //   {
          //     type: 'checkbox',
          //     originType: item.type,
          //     icon: 'icon-check-box',
          //     options: {
          //       inline: false,
          //       defaultValue: [
          //         '编辑'
          //       ],
          //       showLabel: true,
          //       options: [
          //         {
          //           value: '编辑',
          //           label: ''
          //         }
          //       ],
          //       required: false,
          //       requiredMessage: '',
          //       width: '',
          //       remote: false,
          //       remoteType: 'datasource',
          //       remoteOption: item.remoteOption,
          //       remoteOptions: [],
          //       props: {
          //         value: 'value',
          //         label: 'label'
          //       },
          //       remoteFunc: item.remoteFunc,
          //       customClass: '',
          //       labelWidth: 10,
          //       isLabelWidth: true,
          //       hidden: false,
          //       dataBind: true,
          //       disabled: this.editType == 3,
          //       hideLabel: false
          //     },
          //     events: {
          //       onChange: ''
          //     },
          //     name: '',
          //     key: item.key,
          //     model: item.model,
          //     rules: []
          //   };
          //   this.$set(genList, key, obj);
          //   const fieldsObjs = {
          //     name: item.model,
          //     originType: item.type,
          //     englishName: item.model,
          //     fieldType: 'stringType',
          //     length: 999,
          //     defaultValue: '',
          //     remark: '备注',
          //     standby: '',
          //     fieldStatus: 'enable',
          //     valueOrigin: 'fromUser'
          //   };
          //   this.fields.push(fieldsObjs);
          // }

          let obj = {};
          if (item.model && !item.model.endsWith('_noShowCheck')) {
            obj =
            {
              type: 'checkbox',
              originType: item.type,
              originName: item.name,
              icon: 'icon-check-box',
              options: {
                inline: true,
                // defaultValue: [
                //   '编辑'
                // ],
                // showLabel: false,
                showLabel: true,
                // options: [
                //   {
                //     value: '编辑',
                //     label: ''
                //   }
                // ],
                options: [
                  {
                    value: '',
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
                customClass: item.options.customClass,
                labelWidth: 10,
                isLabelWidth: true,
                hidden: false,
                dataBind: true,
                disabled: false,
                // hideLabel: true
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
            // if (item.type == 'table' || item.model.indexOf('label') > -1) { // 字段label开头的是label标签不用选择权限
            if (item.type == 'table' && item.model.indexOf('label') > -1) { // 字段label开头的是label标签不用选择权限
              obj.name = item.name;
              // obj.name = '表格'
            }

            // console.log('item',item)
            this.$set(genList, key, obj);
            const fieldsObjs = {
              name: item.model,
              originType: item.type,
              englishName: item.model,
              widgetName: item.name,
              widgetEl: item.el,
              fieldType: 'stringType',
              length: 999,
              defaultValue: '',
              remark: '备注',
              standby: '',
              fieldStatus: 'enable',
              valueOrigin: 'fromUser'
            };
            this.fields.push(fieldsObjs);
          }
        }
      });
    },
    // 递归获取表单字段
    getAllFieldsList(genList) {
      genList.map((item, key) => {
        if (item.type == 'grid') {
          item.columns.map(val => {
            this.getAllFieldsList(val.list);
          });
        } else if (item.type == 'report') {
          item.rows.map(val => {
            val.columns.map(i => {
              this.getAllFieldsList(i.list);
            });
          });
        } else {
          if (item.model) {
            this.oldFieldsList.push(item.model);
          }
        }
      });
    },
    verifyNodeConfigData(data,parentObj) {
      console.log('verifyNodeConfigData')
      const obj = data;
      const { flowNodeAuditConfig, childFlowNodeTemplate, type } = obj;
      if (type == 'condition' && obj.branchExecuteType != 'custom_choose') {
        // 条件分支
        const { conditionNodes } = obj;
        let conditionFlag1 = false;
        for (var i = 0; i < conditionNodes.length; i++) {
          const item = conditionNodes[i];
          if (this.verifyFlag.length) {
            break;
          }
          if (!item.conditionList.length) {
            this.$message.error((item.name || item.nodeName) + ` 未定义条件字段`);
            this.verifyFlag.push(1);
            break;
          }
          if (item.childFlowNodeTemplate && !this.verifyFlag.length) {
            this.verifyNodeConfigData(item.childFlowNodeTemplate,obj);
          } else {
            conditionFlag1 = true;
          }
        }
        if (conditionFlag1 && !this.verifyFlag.length) {
          this.$message.error(`每一条分支后面必须存在审批节点`);
          this.verifyFlag.push(1);
          return;
        }
      } else if (type == 'parallel') {
        // 并行分支
        obj.parallelNodes.forEach(item => {
          this.verifyNodeConfigData(item.childFlowNodeTemplate,obj);
        });
      } else if (obj.branchExecuteType && obj.branchExecuteType == 'custom_choose') {
        console.log('手动分支')
        // console.log('parentObj', parentObj)
        // console.log('obj', obj)
        // 判断处理期限
        if (parentObj.nodeName == '路由') {
          for (var i = 0; i < parentObj.conditionNodes.length; i++) {
            const item = parentObj.conditionNodes[i];
            if (item.childFlowNodeTemplate.unit && item.childFlowNodeTemplate.delay) {
              this.$message.error(`${item.childFlowNodeTemplate.nodeName}下一个环节为手动分支，不允许配置处理期限`);
              this.verifyFlag.push(1);
              break;
            }
          }
        } else {
          if (parentObj.unit && parentObj.delay) {
            this.$message.error(`${parentObj.nodeName}下一个环节为手动分支，不允许配置处理期限`);
            this.verifyFlag.push(1);
            return;
          }
        }

        // 手动分支
        obj.conditionNodes.forEach(item => {
          this.verifyNodeConfigData(item.childFlowNodeTemplate,obj);
        });
      } else {
        // console.log('普通节点')
        // console.log('parentObj', parentObj)
        // console.log('obj', obj)
        // 判断处理期限（审批人自选）
        if(obj.flowNodeAuditConfig.auditType == 'run_node_choose') {
          if (parentObj.unit && parentObj.delay) {
            this.$message.error(`${parentObj.nodeName}下一个环节的审批方式为审批人自选，不允许配置处理期限`);
            this.verifyFlag.push(1);
            return;
          }
        }
        // 判断处理期限(指定表单人员)
        if(obj.flowNodeAuditConfig.auditType == 'form_person') {
          if (parentObj.unit && parentObj.delay) {
            this.$message.error(`${parentObj.nodeName}下一个环节的审批方式为指定表单人员，不允许配置处理期限`);
            this.verifyFlag.push(1);
            return;
          }
        }

        if (flowNodeAuditConfig && obj.type != 'empty' && obj.type != 'start') {
          if (!flowNodeAuditConfig.auditType) {
            this.$message.error(obj.nodeName + `节点未指定人员`);
            // this.$message.error(obj.nodeName + ` 审批节点未分配权限`);
            this.verifyFlag.push(1);
            return;
          }
        }
        // if (flowNodeAuditConfig && obj.type != 'empty') {
        //   if (!flowNodeAuditConfig.auditType) {
        //     this.$message.error(obj.nodeName + ` 审批节点未分配权限`);
        //     this.verifyFlag.push(1);
        //     return;
        //   }
        // }
      }
      if (childFlowNodeTemplate && !this.verifyFlag.length) {
        this.verifyNodeConfigData(childFlowNodeTemplate,obj);
      }
    },
    verifyData() {
      console.log('verifyData123')
      const nodeConfig = this.$refs.steps3.nodeConfig;
      // const nodeConfig = this.nodeConfig;
      let flag = false;
      if (!nodeConfig.childFlowNodeTemplate) {
        flag = true;
        this.$message.error(`流程设计至少两个审批节点`);
        return flag;
      } else {
        this.verifyNodeConfigData(nodeConfig);
      }
      return flag;
    },
    // 保存流程节点
    saveCustomFlowTemplate() {
      console.log('保存加签')
      console.log('saveCustomFlowTemplate',this.$refs.steps3.nodeConfig)
      console.log('outFlowNodeTemplate',this.outFlowNodeTemplate)
      console.log('this.flowInstanceId',this.flowInstanceId)
      // return;
      this.verifyFlag = [];
      this.verifyFlag.length = 0;

      if (this.verifyData()) {
        return;
      }
      if (this.verifyFlag.length) {
        return;
      }

      this.$axios.post(
        Api.approveManage.updateFlowProxy,
        {
          data:{
            id: this.flowInstanceId
          },
          flowProxyProtocol:{
            data: this.outFlowNodeTemplate
            // data: this.$refs.steps3.nodeConfig
          }
        },
        res => {
          this.loading = false;
          if (res.isSuccess) {
            this.$message.success('保存成功');
            
            this.$emit('saveSuccess',{
              flowNodeProxyId: res.data.currentNodeProxyId,
              flowProxyId: res.data.flowProxyId,
            });
            // this.curruntflowCode = res.message
            // this.flowBackConfigName = data.flowName + '流程回调'
            // this.getCallBackList().then(res => {
            //   if (res.isSuccess) {
            //     this.flowBackList = res.data;
            //     this.callBackDialog = true
            //   }
            // });
          } else {
            this.$message.error(res.message);
          }
        }
      );
      // return;
      // // console.log(' this.$refs.steps3.nodeConfig', this.$refs.steps3.nodeConfig.flowNodeFieldPowerTemplateList.filter(x=>x.formFieldTemplateEnglishName!==""))
      // // this.$refs.steps3.nodeConfig.flowNodeFieldPowerTemplateList = this.$refs.steps3.nodeConfig.flowNodeFieldPowerTemplateList.filter(x=>x.formFieldTemplateEnglishName!=="")
      // this.verifyFlag = [];
      // this.verifyFlag.length = 0;

      // if (this.verifyData()) {
      //   return;
      // }
      // if (this.verifyFlag.length) {
      //   return;
      // }
      // // 子表单里面每列的权限配置是通过建立另一个表格布局控制，并且字段名后面是_col。因为子表单里面的字段没有遍历出来，所以此处进行遍历插入
      // // let copyFormFieldsList = JSON.parse(JSON.stringify(this.fields));
      // // let subFormFieldsList = [];
      // // copyFormFieldsList.forEach( item =>{
      // //   if (item.englishName.endsWith('_col')){
      // //     subFormFieldsList.push({
      // //       name: item.englishName.split('_col')[0],
      // //       originType: 'input',
      // //       englishName: item.englishName.split('_col')[0],
      // //       fieldType: 'stringType',
      // //       length: 999,
      // //       defaultValue: '',
      // //       remark: '备注',
      // //       standby: '',
      // //       fieldStatus: 'enable',
      // //       valueOrigin: 'fromUser'
      // //     })
      // //   }
      // // })

      // const { flowName, groupId, remark, typeId, company } = this.$refs.steps1.infoForm;
      // const {inputFormModelNo} = this.$refs.steps1
      // const data = {
      //   typeId,
      //   flowName,
      //   groupId,
      //   remark,
      //   // 表单模板
      //   // formTemplateVo: {
      //   //   name: this.isNoFormFlow ? this.$refs.steps2.form.name : '',
      //   //   templateData: JSON.stringify(this.cacheTemplateData),
      //   //   fieldsTemplateList: this.fields

      //   // },
      //   // flowNodeTemplate
      //   formTemplateVo: {
      //     name: this.isNoFormFlow ? this.$refs.steps2.form.name : '',
      //     templateData: JSON.stringify(this.cacheTemplateData),
      //     // fieldsTemplateList: copyFormFieldsList.concat(subFormFieldsList)
      //     fieldsTemplateList: this.fields
      //   },
      //   flowNodeTemplate: this.$refs.steps3.nodeConfig,
      //   flowInstanceBizRelevanceList: [
      //     {
      //       otherBiz: 'company',
      //       otherBizId: company
      //     }
      //   ]
      // };
      // if (this.editType == 2 && this.isCopyFlow == 'false') {
      //   data.formTemplateVo.id = this.editDataId;
      //   data.code = this.$route.query.code;
      // }
      // this.loading = true;
      // // 新增
      // let uid //= 'form-'+uuid().substr(0,10)
      // if(inputFormModelNo)uid = inputFormModelNo
      // const formTemplateBizRelevanceList = [
      //   {
      //     otherBiz: 'company',
      //     otherBizId: company // companyId
      //   },
      //   {
      //     otherBiz: 'design',
      //     otherBizId: 'design'
      //   },
      //   {
      //     otherBiz: 'customerCode',
      //     otherBizId: this.$store.state.user.customerCode
      //   }
      // ];
      // if (this.$route.query?.relative) {
      //   formTemplateBizRelevanceList.push({
      //     otherBiz: 'relativeFlow',
      //     otherBizId: 'relativeFlow'
      //   });
      // }
      // if (this.$route.query?.isGeneralFlow) { formTemplateBizRelevanceList.push({ otherBiz: 'isProject', otherBizId: 'isProject' }); }
      // if (this.editType == 1 || this.isCopyFlow == 'true') {
      //   if(uid){
      //     formTemplateBizRelevanceList.push({
      //       otherBiz: 'formModelNo',
      //       otherBizId:uid// 'form-'+uuid().substr(0,10)
      //     });
      //   }
      //   this.$axios.post(
      //     Api.frameworkInfo.departmentFramework.flow.saveCustomFlowTemplate,
      //     {
      //       data,
      //       // formTemplateBizRelevance: {
      //       //   projectId: this.projectId
      //       // }
      //       formTemplateBizRelevanceList
      //     },
      //     res => {
      //       this.loading = false;
      //       if (res.isSuccess) {
      //         this.$message.success('保存成功');
      //         // this.goback();
      //         this.curruntflowCode = res.message
      //         this.flowBackConfigName = data.flowName + '流程回调'
      //         this.getCallBackList().then(res => {
      //           if (res.isSuccess) {
      //             this.flowBackList = res.data;
      //             this.callBackDialog = true
      //           }
      //         });
      //       } else {
      //         this.$message.error(res.message);
      //       }
      //     }
      //   );
      // } else { // 编辑
      //   data.id = this.templateId;
      //   if(!this.formTemplateBizRelevanceVoList.find(el=>el.otherBiz == 'formModelNo')){
      //     this.formTemplateBizRelevanceVoList.push({
      //       otherBiz: 'formModelNo',
      //       otherBizId: uid//'form-'+uuid().substr(0,10)
      //     });
      //   }else{
      //     let find = this.formTemplateBizRelevanceVoList.find(el=>el.otherBiz == 'formModelNo')
      //     find.otherBizId = uid
      //   }
      //   var findCompany = this.formTemplateBizRelevanceVoList.find(el => el.otherBiz == 'company');
      //   findCompany.otherBizId = company; // 编辑时更改公司重新赋值
      //   this.$axios.post(
      //     Api.frameworkInfo.departmentFramework.flow.editCustomFlowTemplate,
      //     {
      //       data,
      //       formTemplateBizRelevanceList:this.formTemplateBizRelevanceVoList
      //     },
      //     res => {
      //       this.loading = false;
      //       if (res.isSuccess) {
      //         this.$message.success('编辑成功');
      //         this.goback();
      //       } else {
      //         this.$message.error(res.message);
      //       }
      //     }
      //   );
      // }
    },
    // 添加固定的表单上传按钮
    genAttachButton(tmp) {
      const list = tmp.list;
      const index = list.findIndex(item => {
        return item.model == 'fileupload_common_file';
      });
      if (index == -1) {
        const staticForm = {
          events: {},
          icon: 'icon-wenjianshangchuan',
          key: 'common_file',
          model: 'fileupload_common_file',
          name: '文件',
          rules: [],
          type: 'fileupload',
          options: {
            defaultValue: [],
            width: '',
            tokenFunc: 'funcGetToken',
            token: '',
            tokenType: 'datasource',
            domain: 'http://tcdn.form.making.link/',
            disabled: false,
            tip: '',
            action: '/api/web/file/api/file/uploadFile',
            customClass: 'fileupload_common_file',
            limit: 9,
            multiple: true,
            isQiniu: false,
            labelWidth: 100,
            hideLabel: true,
            isLabelWidth: false,
            hidden: false,
            dataBind: true,
            headers: [],
            required: false,
            withCredentials: false,
            remoteFunc: 'func_fra8ko0k',
            remoteOption: 'option_fra8ko0k',
            tableColumn: false
          }
        };
        list.push(staticForm);
      }
    }
  }
};
</script>

<style scoped lang="scss">
.flow-container {
  height: 100vh;
  width: 100%;

  .steps-container {
    background: #fff;
    padding: 10px 0 0;
  }

  .flow-content {
    background: #fff;
    margin-top: 10px;
    padding: 10px;
    box-sizing: border-box;
    height: calc(100% - 180px);
    overflow-y: auto;
  }

  .btn-container {
    padding: 10px 0;
    border-top: 1px solid #dcdcdc;
    text-align: center;
    background: #fff;
    position: fixed;
    bottom: 15px;
    width: 100%;
    // margin-top: 10px;
    // padding: 10px 0;
    // border-top: 1px solid #dcdcdc;
    // text-align: center;
    // background: #fff;
  }
  
}
</style>
