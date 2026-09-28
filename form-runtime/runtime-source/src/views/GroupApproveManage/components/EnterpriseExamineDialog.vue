<!--
 * @Descripttion: your project
 * @Author: zhengzetao
 * @Date: 2025-06-23 09:25:11
-->
<!--
 * @Descripttion:
 * @Author: 公司相关类型的表单审核（formMaking制作的表单）
 * @Date: 2021-05-15 05:24:57
-->
<template>
  <div class="dialog-container form-container">
    <!-- :custom-class="btnVisible ? 'dialog-fullscreen examine' : 'dialog-fullscreen'" -->
    <!-- <el-dialog :custom-class="isExamine ? 'dialog-fullscreen isExamine' : 'dialog-fullscreen'" :visible="visible" center
      append-to-body @close='handleClose' width="800px"> -->
    <template v-if="visible">
      <component v-bind:is="currentComponent" :custom-class="isExamine ? 'isExamine' : ''" :visible="visible" center  :style="{zoom: detectZoom() > 140 ? 0.88 : 1}"
        @close='handleClose' :fullscreen="true" :modal="false" :append-to-body="true">
        <!-- append-to-body -->
        <!-- :append-to-body="false" -->
        <!-- <el-tabs type="border-card">
          <el-tab-pane label="表格"> -->
        <div class="examine-content">
          <div class="left-side" id="formContainer" ref="formContainer">
            <!-- 公共上传附件 -->
            <AttachList :models="models" :enableData="enableData" :disabledData="disabledData" :uploading="uploading"
              :btnVisible="btnVisible" :uploadedFile="uploadedFile" :jsonData="jsonData" :isShowList="true"
              :isExamine="isExamine" :isReInitiate="isReInitiate" :flowStatus="flowStatus" ref="attachList"
              v-if="isShowAttachList" />
            <!-- 公共关联流程 -->
            <FormCommonFlowPost ref="flowListSelect" :flowInstanceBizRelevanceList="flowInstanceBizRelevanceList" :initiatorId="initiatorId"
            :enableData="enableData" :isExamine="isExamine" :isReInitiate="isReInitiate" :btnVisible="btnVisible"/>
            <!-- <div @click="test">test</div> -->
            <!-- || visible -->
            <!-- print-read="true" -->
            <fm-generate-form class="exame-form" :data="jsonData" :value="editData" @on-change="onChange" @on-file-upload="onFileUpload"
              ref="generateForm" v-if="visible" :custom-fields="customFields" @openCompanyPersonFramwork="openCompanyPersonFramwork" @openRelationOrganizationDialog="openRelationOrganizationDialog"
              @viewRequestFrom="viewRequestApprovalForm" @viewFrom="viewApprovalForm" :print-read="handleReadStatus" @openGeneralList="openGeneralList" >
            </fm-generate-form>
            <!-- 转发的流程和附言 -->
            <TranspondLog :printStyle="true" v-if="isTranspondFlow" :flowInstanceId="flowInstanceId" :logTableData="logTableData" :transpondFlowData="transpondFlowData" :isNoEnterprise="false"></TranspondLog>
            <!-- 这里加一份附言和日志，用于打印 -->
            <!-- 发起人附言 -->
            <!-- <Postscript :handleReadStatus="handleReadStatus" :flowInstanceId="flowInstanceId" v-bind="$attrs"></Postscript> -->
            <Postscript v-if="printCheckboxList.includes('发起人附言') && handleReadStatus" :handleReadStatus="handleReadStatus" :flowInstanceId="flowInstanceId" v-bind="$attrs"></Postscript>
            <!-- 流程日志 -->
            <FlowLog :printStyle="true" v-if="printCheckboxList.includes('流程日志') && handleReadStatus" :flowInstanceId="flowInstanceId" :logTableData="logTableData" :isNoEnterprise="false"></FlowLog>
            <!-- <FlowLog v-if="printCheckboxList.includes('流程日志') && handleReadStatus" :printStyle="true" :flowInstanceId="flowInstanceId" :logTableData="logTableData" :isNoEnterprise="false"></FlowLog> -->
          </div>
          <!-- 左右收起栏 -->
          <!-- <div class="flex-right expandBar-btn" v-if="showRightSide2" @click="flexRight" :key="1">>></div>
          <div class="flex-left expandBar-btn" v-else @click="flexRight" :key="2">{{'<<'}}</div> -->
          <div class="right-side-wraper" :class="{'noShowWrap': !showRightSide2}" style="position:relative;">
            <div class="flex-right expandBar-btn" v-if="showRightSide2" @click="flexRight" :key="1">>></div>
            <div class="flex-left expandBar-btn" v-else @click="flexRight" :key="2">{{'<<'}}</div>

            <div class="right-side" v-show="showRightSide2">
              <el-button type="primary" style="width:100%;" icon='el-icon-view' @click="handleCheckFlow">查看流程</el-button>
              <!-- 发起人附言 -->
              <Postscript :outSidePostScriptList="postscriptList" :flowInstanceId="flowInstanceId" v-bind="$attrs"></Postscript>
              <!-- 流程日志 -->
              <FlowLog ref="FlowLog" :flowInstanceId="flowInstanceId" :logTableData="logTableData" :isNoEnterprise="false"></FlowLog>

              <eleUpload
                :style="!isExamine && 'margin-bottom:5px'"
                :showOnly="!isExamine"
                ref="eleupload"
                :size="20"
                :attachFile="attachFile"
                style="flex-direction: column;"
              ></eleUpload>
              <!-- <span slot="footer" class="dialog-footer"> -->
              <!-- 审批意见 -->
              <EnterpriseExamineOpinion v-if="visible" :id="flowId" :formId="formId" :businessId="businessId"
                :isExamine="isExamine" :isReInitiate="isReInitiate" :jobTaskId="jobTaskId" :flowNodeType="flowNodeType"
                :nextNodeProxyId="nextNodeProxyId" @postExamine="postExamine" @getFormPersonValue="getFormPersonValue" :initiatorId="initiatorId"
                :auditPassLogicFlag="auditPassLogicFlag" :nextNodeName="nextNodeName" :currentPendingNodeName="currentPendingNodeName" :flowInstanceId="flowInstanceId"
                v-bind="$attrs" v-on="$listeners" :flowName="flowName" :selectFlowType="selectFlowType" @print="print" @handlePrint="handlePrint" @contractPayClose="contractPayClose"
                :flowInstanceBizRelevanceList="flowInstanceBizRelevanceList" :flowNodeProxyId="flowNodeProxyId" :isTranspondFlow="isTranspondFlow"
                :outTracking.sync="outTracking" :flowProxyId="flowProxyId" @updateList="updateList" :isTimeoutPage="isTimeoutPage" :hideTrackingButton="hideTrackingButton" @bindFile="bindFile">
              </EnterpriseExamineOpinion>
               <!-- :outFlowNodeTemplate="allFlowNodeDetail" -->
              <!-- </span> -->
            </div>
          </div>
        </div>
      </component>
    </template>
    <IndicatorHeaderDialog :visible.sync="indicatorHeaderVisible" :fielSelectType="fielSelectType" :companyId="companyId" :selectUserCompanyId="selectUserCompanyId" :departmentId="departmentId"
      v-if="indicatorHeaderVisible" @selectHeader="selectHeader" :isRelative="isRelative"/>
      <!-- 集团人管岗位关联选择 -->
      <RelationOrganizationDialog v-if="relationOrganizationVisible" :visible.sync="relationOrganizationVisible" :fielSelectType="fielSelectType" @selectValue="selectValue" ></RelationOrganizationDialog>
      <!-- c查看关联转正审批表 -->
    <ApprovalFormDialog v-if="viewForm.visible" :visible.sync="viewForm.visible" :flowInstanceId="viewForm.flowInstanceId" :formId="viewForm.formId"></ApprovalFormDialog>
    <!-- 公共列表 -->
    <generalList v-if="generalVisible" :visible.sync="generalVisible" :config="config" @confirmChoose="confirmChoose"></generalList>

    <CheckFlowNodeDetail v-if="checkViewFlowDetailVisible" :dialogVisible.sync="checkViewFlowDetailVisible"
      :flowInstanceId="flowInstanceId" :flowId="flowId" :initiatorId="initiatorId" :companyId="companyId"></CheckFlowNodeDetail>
  </div>
</template>

<script>
import eleUpload from '@/components/EleUpload';
import AttachList from '@/components/AttachList/index.vue';
import DyTable from '@/components/DyTable';
import EnterpriseExamineOpinion from './EnterpriseExamineOpinion';
import FlowLog from './flowLog';
import TranspondLog from './TranspondLog';
import Postscript from './Postscript.vue';
import Api from '@/api';
import mixin from './mixin'
import customJson from '@/components/Custom/customJson'
// import IndicatorHeaderDialog from '@/views/GroupApproveManage/Submitted/components/IndicatorHeaderDialog.vue'
import IndicatorHeaderDialog from '@/components/IndicatorHeaderDialog.vue'
import RelationOrganizationDialog from '@/components/RelationOrganizationDialog.vue'
import { localstorageSet,localstorageGet } from '@/utils/auth';
import { detectZoom } from '@/utils';
import ApprovalFormDialog from '../Submitted/components/ApprovalFormDialog.vue'
import FormCommonFlowPost from '@/views/GroupApproveManage/components/FormCommonFlowPost/index.vue'
import generalList from '@/components/generalList.vue'
import { Print as $print } from '@/utils/print.js';
import { deepClone } from '../../../utils';
import CheckFlowNodeDetail from './CheckFlowNodeDetail.vue';
export default {
  name: 'EnterpriseExamineDialog',
  components: { AttachList, DyTable, EnterpriseExamineOpinion, FlowLog, Postscript, IndicatorHeaderDialog, ApprovalFormDialog,
  eleUpload,RelationOrganizationDialog,generalList,FormCommonFlowPost,TranspondLog,CheckFlowNodeDetail },
  mixins:[mixin],
  props: {
    operaType: { // 新增：add，编辑：edit reEdit，查看：check
      type: String,
      default: ''
    },
    actionType: { // preview edit examine  create
      type: String,
      default: ''
    },
    visible: {
      type: Boolean,
      default: false
    },
    showRightSide: {
      type: Boolean,
      default: true
    },
    isExamine: {
      type: Boolean,
      default: true
    },
    lastCountersignFlag: {
      type: Boolean,
      default: true
    },
    auditPassLogicFlag: {
      type: Boolean,
      default: true
    },
    isReInitiate: {
      type: Boolean,
      default: false
    },
    initiatorId: {
      type: String,
      default: ''
    },
    showFlowLog: {
      type: Boolean,
      default: true
    },
    formId: {
      type: String,
      default: ''
    },
    flowId: {
      type: String,
      default: ''
    },
    businessId: {
      type: String,
      default: ''
    },
    companyId: {
      type: String,
      default: ''
    },
    flowNodeProxyId: {
      type: String,
      default: ''
    },
    flowInstanceId: {
      type: String,
      default: ''
    },
    batchNo: {
      type: String,
      default: ''
    },
    jobTaskId: {
      type: String,
      default: ''
    },
    btnVisible: {
      type: Boolean,
      default: true
    },
    flowNodeType: {
      type: String,
      default: ''
    },
    nextNodeName: {
      type: String,
      default: ''
    },
    currentPendingNodeName: {
      type: String,
      default: ''
    },
    nextNodeProxyId: {
      type: String,
      default: ''
    },
    isDialog: {
      type: Boolean,
      default: true
    },
    flowName: {
      type: String,
      default: ''
    },
    selectFlowType:{
      type: String,
      default:''
    },
    flowStatus:{
      type: String,
      default:''
    },
    flowProxyId:{
      type: String,
      default:''
    },
    flowInstanceBizRelevanceList:{
      type: Array,
      default:function(){
        return [];
      }
    },
    tracking:{
      type: Boolean,
      default: false
    },
    hideTrackingButton: {
      type: Boolean,
      default: false
    },
    isTimeoutPage:{ //是否是超时跳过的页面
      type:Boolean,
      default:false
    }
  },
  provide() {
    return {
      enterpriseExamineDialog: this, // 传递到自定义组件使用zxf inject: ['flowDialog', 'enterpriseExamineDialog'],
      flowDialog: this
    };
  },
  data() {
    return {
      showRightSide2: this.showRightSide,
      isTranspondFlow: false, // 是否是转发流程
      detectZoom,
      handleReadStatus: false,
      // handleReadStatus: true,
      attachFile: [],
      indicatorHeaderVisible: false,
      currentField: '',
      currentRowIndex: '',
      currentTable: '',
      fielSelectType: '',
      selectData: null,
      customFields: customJson,
      active: 0,
      stepData: [],
      jsonData: {},
      editData: {},
      checkboxRersonGroup: [],
      nextAuditorList: [],
      nodeChooseVisible: false,
      personList: [], // 流程人员列表
      models: {},
      enableData: [],
      disabledData: [],
      uploading: [],
      uploadedFile: {},
      logColKey: {
        executorName: {
          label: '操作人',
          width: '200px'
        },
        createDate: {
          label: '操作时间',
          width: '200px'
        },
        auditStatus: {
          label: '操作状态',
          width: '200px'
        },
        executeDesc: {
          label: '操作描述',
          handle: (scope, createElement) => {
            return createElement('div', {
              style: {
                whiteSpace: 'pre-wrap!important'
              }
            }, scope.row.executeDesc);
          }
        }
      },
      logTableData: [],
      approveForm: {
        approveMessage: '' // 审批信息
      },
      rules: {
        approveMessage: [
          { required: true, message: '请填写审批意见', trigger: 'blur' }
        ]
      },
      isShowFlowLog: true,
      setId:{
        user_name:'user_id',
        company_name:'company_id',
        department_name:'department_id',
        post_name:'post_id',
        mentor_name:'mentor_id',
        department_manager_name:'department_manager_id',
      },
      viewForm:{
        flowInstanceId:'',
        formId:'',
        visible:false
      },
      selectUserCompanyId:'',
      departmentId:'',
      isPrint:false,
      relationOrganizationVisible:false,
      printCheckboxList:[],
      postscriptList:[],
      generalVisible:false,
      isRelative:true,
      config:{},
      checkViewFlowDetailVisible:false,
      transpondFlowData:[],
      checkFlowNode:[],
      isScoreFlag:false,
      outTracking:'', //外部传入的流程跟踪状态
      flowNodeTemplate:[],
      // allFlowNodeDetail:{},
      // transpondFlowData:[
      //   {
      //     yijian:[
      //       {
      //         executorName:'郑举涛',
      //         auditStatus:'通过',
      //         createDate:'2025-03-07 10:25:52',
      //         executeDesc:'流程发起',
      //       },
      //       {
      //         executorName:'郑举涛2',
      //         auditStatus:'通过',
      //         createDate:'2025-03-07 10:25:52',
      //         executeDesc:'流程发起',
      //       },
      //       {
      //         executorName:'郑举涛3',
      //         auditStatus:'通过',
      //         createDate:'2025-03-07 10:25:52',
      //         executeDesc:'流程发起',
      //       },
      //     ],
      //     fuyan:[
      //       {
      //         sendName:'郑举涛',
      //         createDate:'2025-03-07 10:25:52',
      //         text:'通过',
      //         relationFileDataVos: [
      //           {
      //             id:'ad467975e30407f81204bf614dbe81d',
      //             originFileName:'offer邮件格式.docx',
      //             fileId:'328ee6672e36465e81d07913c1906be9',
      //             fileHame:'offer邮件格式(1111116).docx',
      //             fileUrl:'',
      //           }
      //         ],
      //       }
      //     ]
      //   },
      //   {
      //     yijian:[],
      //     fuyan:[],
      //     approveForm:{
      //       approveMessage: '法法师法式复古',
      //       personName: '佳佳'
      //     }
      //   }
      // ]
    };
  },
  computed: {
    currentComponent() {
      if (this.isDialog) {
        return 'el-dialog'
      } else {
        return 'div'
      }
    },
    isShowAttachList(){
      const noShowFileType=['travel_expense','expense_loan','expense_repayment','request_funds','contract_compliance_review','contract_seal_review','expense_budget_noBelong']
      // this.selectFlowType =='travel_expense'||
      if(noShowFileType.includes(this.selectFlowType)){
        return false
      }else{
        return true
      }
    }
  },
  watch: {
    tracking:{
      handler(val){
        this.outTracking = val
      },
      immediate: true
    },
    // // 加签bug调试暂时注释
    // flowNodeProxyId: {
    //   handler(val) {
    //     console.log('flowNodeProxyId updated:', val);
    //     // 当flowNodeProxyId发生变化时，更新内部变量
    //     // 这样可以确保加签后flowNodeProxyId得到更新
    //   },
    //   immediate: true
    // },
    // // 加签bug调试暂时注释
    // visible: {
    //   handler(val) {
    //     if (val) {
    //       // 当弹窗打开时，重新获取数据
    //       this.getFormDetail();
    //       this.fetchLogData();
    //       this.getAttachmentList();

    //       // 如果是审核状态，重新获取流程节点信息
    //       if (this.isExamine) {
    //         this.getFlowDetailFindFormPerson();
    //       }
    //     }
    //   },
    //   immediate: true
    // },
    jsonData: {
      handler(val) {
        console.log(val,'++++++++++++')
        console.log('this.isTranspondFlow',this.isTranspondFlow)
        if (val.list) {
          this.$nextTick(() => {
            console.log('val')
            console.log('this.selectFlowType--1', this.selectFlowType)
            // if (this.selectFlowType == 'contract_compliance_review' || this.isTranspondFlow) {
            if (this.selectFlowType == 'contract_compliance_review') {
              // 合同合规性表单-表单控制自定义字段
              this.$refs.generateForm.setOptions(['custom_contractLegalField'], {
                extendProps: {
                  isFlowInitiate: false,
                  businessId: this.businessId,
                  companyId: this.companyId,
                  isExamine: this.isExamine, // 是否审核状态
                  isReInitiate: this.isReInitiate, // 是否重新发起草稿状态
                  isTranspondFlow: this.isTranspondFlow, // 是否是转发数据
                }
              })
              setTimeout(x => {
                let custom_contractLegalField = this.$refs.generateForm.getComponent('custom_contractLegalField') // 合同合规评审表中自定义的业务组件
                console.log('custom_contractLegalField', custom_contractLegalField)
                custom_contractLegalField.initFormInstance(this.$refs.generateForm);
              }, 1000)
            } else if (this.selectFlowType == 'contract_seal_review') { // 合同盖章评审
              console.log(1111111111111,'合同盖章评审')
              console.log(this.businessId,'合同盖章评审')
            // } else if (this.selectFlowType == 'contract_seal_review' || this.isTranspondFlow) { // 合同盖章评审
              localstorageSet('isValueInit', true); // 盖章表单由于初始化有赋值，所以在合同主体处需要这个进行判断
              // 合同盖章评审表单-表单控制自定义字段
              this.$refs.generateForm.setOptions(['custom_contractSealField'], {
                extendProps: {
                  isFlowInitiate: false,
                  businessId: this.businessId,
                  companyId: this.companyId,
                  // isExamine: this.isExamine, // 是否审核状态
                  // isReInitiate: this.isReInitiate, // 是否重新发起草稿状态
                }
              })
              setTimeout(x => {
                // 赋值最新盖章合同文件
                let custom_contractSealField = this.$refs.generateForm.getComponent('custom_contractSealField') // 合同盖章评审表中自定义的业务组件
                console.log('custom_contractSealField', custom_contractSealField)
                custom_contractSealField.initFormInstance(this.$refs.generateForm);
                // custom_contractSealField.initContractFile_inAudit(this.$refs.generateForm);
              }, 200)
              if (!this.isExamine && !this.isReInitiate) { // 两个状态为false时，代表已办的查看和已发的详情
                setTimeout(x => {
                  // 赋值最新盖章合同文件
                  let custom_contractSealField = this.$refs.generateForm.getComponent('custom_contractSealField') // 合同盖章评审表中自定义的业务组件
                  console.log('custom_contractSealField', custom_contractSealField)
                  custom_contractSealField.getContractInfo_check_success(this.$refs.generateForm);
                }, 200)
              }
            }
          });
        }
      },
      immediate: true
    },
    // visible(val) {
    //   if (val) {
    //     this.getFormDetail();
    //     this.fetchLogData();
    //   }
    // }
  },

  async created() {
    console.log('this.flowProxyId12',this.flowProxyId)
    this.checkFlowNode = await this.getAuditRecord();
    this.getFormDetail();
    this.fetchLogData();
    this.getAttachmentList();
    console.log('是否审核阶段',this.isExamine)
    console.log('是否重新发起阶段',this.isReInitiate)
    console.log('this.batchno1111', this.batchNo)

    this.isExamine && this.getFlowDetailFindFormPerson();

    // 加签bug调试暂时注释
    // 监听来自Backlog的数据更新通知
    // this.$bus.$on('updateExamineDialogData', this.handleUpdateDialogData);
  },
  mounted() {
    console.log('审核流程-mounted', this.currentPendingNodeName)
    console.log('isExamine',this.isExamine)
    console.log('isReInitiate',this.isReInitiate)
    console.log('=====businessId=====',this.businessId)
    // console.log('2222',this.flowName,this.selectFlowType,this.businessId)
    // this.calcRight();
    // var i = 0;
    // var inter = setInterval(() => {
    //   if (++i == 10) {
    //     clearInterval(inter);
    //   }
    //   this.calcRight();
    // }, 500);
    // window.addEventListener('resize', this.calcRight);
  },
  // 加签bug调试暂时注释
  // destroyed() {
  //   // 移除事件监听
  //   this.$bus.$off('updateExamineDialogData', this.handleUpdateDialogData);
  // },
  methods: {
    flexRight() {
      if (this.showRightSide2) {
        this.showRightSide2 = false;
      } else {
        this.showRightSide2 = true;
        // this.calcRight();
      }
    },
    // 加签bug调试暂时注释
    // 强制刷新数据的方法
    // forceRefresh() {
    //   this.getFormDetail();
    //   this.fetchLogData();
    //   this.getAttachmentList();
    //   console.log('强制刷新数据完成');
    // },
    // // 处理来自Backlog组件的数据更新通知
    // handleUpdateDialogData(updatedRow) {
    //   console.log('接收到更新数据通知:', updatedRow);

    //   // 更新相关属性
    //   if (updatedRow.flowNodeProxyId) {
    //     this.flowNodeProxyId = updatedRow.flowNodeProxyId;
    //   }

    //   if (updatedRow.nextNodeProxyId) {
    //     this.nextNodeProxyId = updatedRow.nextNodeProxyId;
    //   }

    //   if (updatedRow.jobTaskId) {
    //     this.jobTaskId = updatedRow.jobTaskId;
    //   }

    //   // 更新其他可能变化的属性
    //   this.currentPendingNodeName = updatedRow.currentPendingNodeName || this.currentPendingNodeName;
    //   this.nextNodeName = updatedRow.nextNodeName || this.nextNodeName;

    //   console.log('更新后的flowNodeProxyId:', this.flowNodeProxyId);
    // },
    // 用于合同付款单的冻结金额，审批人自选时候重复提交，导致冻结金额无法清空的问题
    contractPayClose(){
      if (this.selectFlowType == 'contract_payment_form') { // 合同付款单
        let url = '/web/measuring/api/contractPaymentForm/update';
        var values = this.$refs.generateForm.getValues();
        console.log('values',values)
        let allValues= JSON.parse(JSON.stringify(values));
        const expenseBudgetList = allValues.expenseBudgetList.map(item=>{
          return {
            allChildId:item.departmentId.join(),
            budgetId:item.departmentId.length > 1 ? item.departmentId[item.departmentId.length - 1] : '',
            mainId: item.departmentId[1],
            departmentId: item.departmentId[0],
            money: item.money,
            type:2, // 公司预算固定传2
          }
        })
        let data =  {
          "id": this.businessId,
          "status": '0',//数据0草稿1提交
          "examineStatus": "0",//流程状态：固定传0
          "expenseBudgetVoList": expenseBudgetList//归口信息
        }

        this.$axios.post(
          url, {
          data: data
        }).then(res => {
          if (res.isSuccess) {
          }
        });
      }

      console.log('close3')
    },
    getFlowDetailFindFormPerson() { // 获取下个节点表单人员字段List
      console.log('getFlowDetailFindFormPerson-方法')
      console.log('this.$attrs.clickRow',this.$attrs.clickRow)
      const url = this.$attrs.clickRow.flowInstanceId ? Api.schedule.getFlowInstanceTemplateNode : Api.schedule.flowTemplateFindById;
      console.log('getFlowDetailFindFormPerson-url',url)
      this.$axios.post(url, { data: { id: this.$attrs.clickRow.flowProxyId }},
        res => {
          if (res.isSuccess) {
            // this.allFlowNodeDetail = res.data;
            var data = res.data.flowNodeTemplate;
            this.flowNodeTemplate = JSON.parse(JSON.stringify(data));
            var nextNodeProxyId = this.$attrs.clickRow?.nextNodeProxyId;
            console.log(nextNodeProxyId, 'nextNodeProxyId--nextNodeProxyId');
            var fields = [];
            var fuc = (j) => {
              if (j.id == nextNodeProxyId) {
                // nextNode = j; // 匹配上了下个节点
                if (j.flowNodeAuditConfig?.auditType == 'form_person') {
                  console.log('form_person1')

                  fields.push({
                    bizId: j.flowNodeAuditConfig.formPersonFields,
                    nodeProxyId: j.id
                  });
                  console.log('fields',fields)
                }
                if (j?.conditionNodes && j?.conditionNodes.length) {
                  j.conditionNodes.forEach(c => {
                    if (c.childFlowNodeTemplate.flowNodeAuditConfig.auditType == "form_person") {
                      console.log('form_person2')
                      console.log('fields',fields)
                      fields.push({
                        bizId: c.childFlowNodeTemplate.flowNodeAuditConfig.formPersonFields,
                        nodeProxyId: c.childFlowNodeTemplate.id
                      });
                    }
                  });
                }
                if (j?.parallelNodes && j?.parallelNodes.length) {
                  j.parallelNodes.forEach(p => {
                    if (p.childFlowNodeTemplate.flowNodeAuditConfig.auditType == "form_person") {
                      console.log('form_person3')
                      console.log('fields',fields)
                      fields.push({
                        bizId: p.childFlowNodeTemplate.flowNodeAuditConfig.formPersonFields,
                        nodeProxyId: p.childFlowNodeTemplate.id
                      });
                    }
                  });
                }
              } else {
                if (j.parallelNodes && j.parallelNodes.length) {
                  j.parallelNodes.forEach(p => {
                    if (p.id == nextNodeProxyId) {
                      if (p.childFlowNodeTemplate.flowNodeAuditConfig?.auditType == 'form_person') {
                        console.log('form_person4')
                        console.log('fields',fields)
                        fields.push({
                          bizId: p.childFlowNodeTemplate.flowNodeAuditConfig.formPersonFields,
                          nodeProxyId: p.id
                        });
                      }
                    } else {
                      p.childFlowNodeTemplate && fuc(p.childFlowNodeTemplate);
                    }
                  });
                }
                if (j.conditionNodes && j.conditionNodes.length) {
                  j.conditionNodes.forEach(c => {
                    if (c.id == nextNodeProxyId) {
                      if (c.childFlowNodeTemplate.flowNodeAuditConfig?.auditType == 'form_person') {
                        console.log('form_person5')
                        console.log('fields',fields)
                        fields.push({
                          bizId: c.childFlowNodeTemplate.flowNodeAuditConfig.formPersonFields,
                          nodeProxyId: c.id
                        });
                      }
                    } else {
                      c.childFlowNodeTemplate && fuc(c.childFlowNodeTemplate);
                    }
                  });
                }
                j.childFlowNodeTemplate && fuc(j.childFlowNodeTemplate);
              }
            };
            fuc(data);
            this.getFlowDetailFindFormPerson.fields = fields;
            console.log(this.getFlowDetailFindFormPerson.fields, 'fields--fields--fields');
            window.abb5 = this;
          }
        }
      );
    },
    // 是否JSON
    isJSON(str) {
      try {
        const result = JSON.parse(str);
        // JSON 格式必须是一个对象或数组
        return typeof result === 'object' && result !== null;
      } catch (e) {
        return false;
      }
    },
    // getFormPersonValue(cb) {
    //   console.log('getFormPersonValue1')
    //   var values = this.$refs.generateForm.getValues();
    //   var nextAuditorList = [];
    //   this.getFlowDetailFindFormPerson.fields.forEach(field => {
    //     console.log('进来',this.getFlowDetailFindFormPerson.fields)
    //     if (values[field.bizId]) {
    //       // 兼容两种人员选择器
    //       if (this.isJSON(values[field.bizId])) { // 如果是json字符串，需要进行转换
    //         nextAuditorList.push({ bizId: JSON.parse(values[field.bizId]).id, nodeProxyId: field.nodeProxyId });
    //       } else {
    //         nextAuditorList.push({ bizId: values[field.bizId], nodeProxyId: field.nodeProxyId });
    //       }
    //       // nextAuditorList.push({ bizId: values[field.bizId], nodeProxyId: field.nodeProxyId });
    //     }
    //   });
    //   cb(nextAuditorList);
    // },
    getFormPersonValue(param) {
      console.log('getFormPersonValue2')
      console.log('this.getFlowDetailFindFormPerson',this.getFlowDetailFindFormPerson.fields)
      var values = this.$refs.generateForm.getValues();
      this.getFlowDetailFindFormPerson.fields.forEach(field => {
          let key = field.bizId
          if(values[key] === undefined) {
            let realField = key.replace('__formPersonId','')
            if (this.isJSON(values[realField])) {
              let obj = JSON.parse(values[realField])
              param.formDataMongoVo.data[key] = obj.id
            }else{
              param.formDataMongoVo.data[key] = values[realField]
            }
          }
      });
    },
    confirmChoose(chooseData){
      if(typeof(chooseData) == 'object'){
        this.$refs.generateForm.setData({[this.config.field]:JSON.stringify(chooseData)});
      }else{
        this.$refs.generateForm.setData({[this.config.field]:chooseData});
      }
    },
    openGeneralList(obj){
      this.config = obj
      this.generalVisible = true
    },
    print(bool){
      this.isPrint = bool
    },
    handlePrint(data){
      console.log('handlePrint',data)
      this.printCheckboxList = data;
      this.handleReadStatus = true;

      setTimeout(x=>{
        $print(this.$refs.formContainer); // 2025.1.2用新的打印插件
      },1000)
      // },500)

      setTimeout(x=>{
        this.handleReadStatus = false;
      },1200)
      // setTimeout(x=>{
      //   this.handleReadStatus = false;
      // },1500)
    },
    getAttachmentList(id = this.flowInstanceId) { // 1、根据业务id获取附件文件回显
      this.$axios.post(
        Api.schedule.getAttachmentList, {
        data: {
          relationId: id
        }
      }).then(res => {
        if (res.isSuccess) {
          const list = res.data;
          const attachFile = list?.map(item => {
            return {
              id: item.fileId,
              fileName: item.fileName,
              fileUrl: item.fileUrl,
              absolutelyFileUrl: item.fileUrl
            };
          });
          this.attachFile = attachFile || [];
        }
      });
    },
    bindFile(data){
      if(data.flowJobTaskId){
        this.bindBatchFileByIds(data.flowJobTaskId)
      }
    },
    bindBatchFileByIds(relationId = this.flowInstanceId) { // 多个文件绑定业务id
      const fileIds = this.$refs.eleupload.getFileId();
      if (fileIds && fileIds.length) {
        const data = {
          relationId,
          fileIds
        };
        this.$axios.post(
          Api.budgetManage.saveBatchFile,
          { data }
        );
      }
    },
    // //查询试用期员工转正审批表的得分数据
    // getProbationEmployeeConfirmationApprovalScores(){
    //   this.$axios.post(
    //     '/web/formData/getProbationEmployeeConfirmationApprovalScores',
    //     {
    //       data: {
    //         flowInstanceId: this.flowInstanceId
    //       }
    //     },
    //     (res) => {
    //       if (res.isSuccess) {
    //         this.$nextTick(() => {
    //           this.$refs.generateForm.setData({ daily_score: res.data['日常得分'] });
    //           this.$refs.generateForm.setData({ report_score: res.data['转正述职得分'] });
    //           this.$refs.generateForm.setData({ total_score: res.data['最终得分'] });
    //         });
    //       }
    //     }
    //   );
    // },
    handleCheckFlow() {
      // console.log('查看流程')
      // console.log('查看流程',this.$refs.generateForm.getValues())
      // var pt = this.$parent;
      // pt.handleCheckFlow(pt.clickRow);
      this.checkViewFlowDetailVisible = true;

    },
    selectHeader(data, selectCompany) { // 人员选择选择赋值
      // this.selectData = data;
      // this.$refs.generateForm.setData({ handleCompany: selectCompany });
      // if (this.currentTable) { // 子表单赋值
      //   const tableArr = this.$refs.generateForm.getValue(this.currentTable);
      //   tableArr[this.currentRowIndex][this.currentField] = data.name;
      //   const obj = {};
      //   obj[this.currentTable] = tableArr;
      //   this.$refs.generateForm.setData(obj);
      // } else {
      //   const obj = {};
      //   obj[this.currentField] = data.name;
      //   obj[this.currentField + '_id'] = data.id; // 赋值id审批时选择表单人员(字段以_id结尾)
      //   this.$refs.generateForm.setData(obj);
      // }
      this.selectData = data;
      if(selectCompany){
        this.$refs.generateForm.setData({ handleCompany: selectCompany.name });
      }
      // 员工试用期考核表用
      console.log(this.currentField,'this.currentField++++++')
      if(this.currentField=='company_name'){
        if(data.type==1){
          this.$refs.generateForm.setData({ currentCompanyId: data.id });
        }else{
          this.$refs.generateForm.setData({ currentCompanyId: selectCompany.id });
        }
      }
      if(this.currentField=='departmentName'){
        this.$refs.generateForm.setData({ departmentId: data.id });
      }
      if(this.currentField=='company_dept'){
        let obj = {},currentField = 'userName'
        if(data.type == 1 && selectCompany.type == 1){//这个是选择公司
          obj['type'] = 'company'
          obj[currentField + '_dept'] = '';
          obj[currentField + '_deptid'] = '';
          obj[currentField + '_company'] = data.name;
          obj[currentField + '_companyid'] = data.id;
        }
        if(data.type == 2 && selectCompany.type == 1){ //选择部门
          obj['type'] = 'dep'
          obj[currentField + '_dept'] = data.name;
          obj[currentField + '_deptid'] = data.id;
          obj[currentField + '_company'] = selectCompany.name;
          obj[currentField + '_companyid'] = selectCompany.id;
        }
        this.$refs.generateForm.setData(obj);
        return
      }
      if (this.currentTable) { // 子表单赋值
        console.log(this.currentField,'this.currentField')
        console.log(this.setId[this.currentField],'this.currentField')
        const tableArr = this.$refs.generateForm.getValue(this.currentTable);
        tableArr[this.currentRowIndex][this.currentField] = data.name;
        // 设置id
        if(this.setId.hasOwnProperty(this.currentField)){
          tableArr[this.currentRowIndex][this.setId[this.currentField]] = data.id;
          if(this.currentField=='company_name'||this.currentField=='department_name'){
            tableArr[this.currentRowIndex]['post_name'] = ''
            tableArr[this.currentRowIndex]['post_id'] = ''

          }
        }
        const obj = {};
        obj[this.currentTable] = tableArr;
        this.$refs.generateForm.setData(obj);
      } else {
        console.log(8888,this.currentField)
        const obj = {};
        obj[this.currentField] = data.name;
        obj[this.currentField + '_id'] = data.id; // 赋值id审批时选择表单人员(字段以_id结尾)
        this.$refs.generateForm.setData(obj);
        // 设置id
        if(this.setId.hasOwnProperty(this.currentField)){
          console.log(99999,this.currentField)
          this.$refs.generateForm.setData({[this.setId[this.currentField]]:data.id});
          if(this.currentField=='company_name'||this.currentField=='department_name'){
            const restObj = {
              post_name:'',
              post_id:'',
            }
            this.$refs.generateForm.setData(restObj);
          }
        }
      }

      // 人员增补审批表  获取
      if(this.fielSelectType=='duty'&&this.setDataProps){
        this.$axios.post(
        Api.frameworkInfo.getDutyList,
        {
          data: {
            departmentId: data.parentId
          }
        },
        res => {
          if (res.isSuccess) {
            const currentPost = res.data.filter(item=>{
              return item.id == data.id
            })
            this.$refs.generateForm.setData({[this.setDataProps]:currentPost[0]?.total});
            console.log(currentPost,'currentPost++')
          } else {
            this.$message.error(res.message);
          }
        }
        )

      }
    },
    selectValue(post,department,company){
      if(this.currentField=='post_name'){
        const obj = {
          post_name:post.name,
          post_id:post.id,
          department_name:department.name,
          department_id:department.id,
          company_name:company.name,
          company_id:company.id,
        }
        this.$refs.generateForm.setData(obj);
      }
      if(this.currentField=='company_department_post_name'){
        const obj = {
          company_department_post_name:company.name+'/'+department.name+'/'+post.name,
          post_id:post.id,
          department_id:department.id,
          company_id:company.id,
        }
        this.$refs.generateForm.setData(obj);
      }
    },
    openRelationOrganizationDialog(data){
      console.log(77777)
      const { field, rowIndex, table,group } = data.argument;
      this.selectUserCompanyId = data.userCompanyId
      this.fielSelectType = data.fielSelectType;
      this.companyId =data.companyId||'';
      this.setDataProps = data.setData||'';
      this.currentField = field;
      this.currentRowIndex = rowIndex;
      this.currentTable = table||group;
      this.relationOrganizationVisible = true;
    },
    openCompanyPersonFramwork(data, data2) {
      console.log(data, 'da233');
      // const { field, rowIndex, table,group } = data.argument;
      // this.fielSelectType = data.fielSelectType;
      // this.currentField = field;
      // this.currentRowIndex = rowIndex;
      // this.currentTable = table||group;
      // this.indicatorHeaderVisible = true;
      const { field, rowIndex, table,group } = data.argument;
      this.selectUserCompanyId = data.userCompanyId
      this.fielSelectType = data.fielSelectType;
      this.companyId = this.$refs.generateForm.getValue('currentCompanyId')||data.companyId||'';
      this.setDataProps = data.setData||'';
      this.currentField = field;
      this.currentRowIndex = rowIndex;
      this.currentTable = table||group;
      if(this.fielSelectType=='duty'){ //岗位部门关联
        console.log(this.$refs.generateForm.getValues())
        if(this.currentTable){

         const  currentTable = this.$refs.generateForm.getValue(this.currentTable)
         console.log(this.currentTable,this.currentRowIndex,'999999999999',currentTable)
          this.departmentId = currentTable[this.currentRowIndex].department_id
        }else{
          this.departmentId = this.$refs.generateForm.getValue('department_id')||this.$refs.generateForm.getValue('company_id')
        }
      }else{
        this.departmentId = ''
      }
      this.indicatorHeaderVisible = true;
    },
    viewApprovalForm(data){
      console.log('data',data)
      if(this.selectFlowType == 'expense_guarantee_repayment'){
        this.viewRequestApprovalForm(data);
        return;
      }
      const parmas = {
          flowName: '',
          useScope: 'invest',
          auditWayList: [],
          // statusList:['run'],
          id:data.id,//data.formProxyId,
          initiator:"all",
          // flowInstanceBizRelevanceList: [
          //   {
          //     otherBiz: 'expense_loan',
          //     otherBizId:data.formProxyId
          //   },
          // ],
        };
        //差旅报销单查看
        if(this.selectFlowType == 'travel_expense'){
          parmas.flowInstanceBizRelevanceList = [
            {
              otherBiz: 'expense_loan',
              otherBizId:data?.argument?.row?.expenseReimbursementId
            }
          ]
        }
        if(this.selectFlowType == 'assets_buy_apply' || this.selectFlowType == 'assets_transfer'){
          parmas.initiator = "all"
          parmas.id = data.id
          parmas.auditWayList= []
          parmas.statusList = ['run','end']
        }
        this.$axios.post(
          Api.schedule.getFlowInstanceList,
          {
            data:parmas,
            pagination: true,
            pages: 1,
            size: 99
          },
          res => {
            if (res.isSuccess) {
              const flow = res.data[0];
              // this.selectFlowType = flow.auditWay;
              // this.isExamine = false;
              // this.isReInitiate = false;
              // this.flowId = flow.flowProxyId;
              // this.formId = flow.formProxyId;
              // this.flowNodeProxyId = flow.currentNodeProxyId;
              // this.flowInstanceId = flow.id;
              // this.jobTaskId = flow.jobTaskId;
              // this.examineDialogVisible = true;
              // const find = flow.flowInstanceBizRelevanceList.find(item => item.otherBiz == flow.auditWay);
              // this.businessId = find?.otherBizId || '';
              this.viewForm.formId = flow?.formProxyId
              this.viewForm.flowInstanceId = flow?.id;
              this.viewForm.visible = true
            } else {
              this.$message.error(res.message);
            }
          }
        );
      // if(data.argument&&data.argument.table){
      //   if(data.argument.table=='accountDetailedVoList'){
      //     const tableData = this.$refs.generateForm.getValue("accountDetailedVoList")
      //     this.viewForm.formId = tableData[data.argument.rowIndex].formProxyId
      //     this.viewForm.flowInstanceId = tableData[data.argument.rowIndex].processId
      //   }
      // }else{
      //   this.viewForm.formId = this.$refs.generateForm.getValue("probation_employee_approval_form_formProxyId")
      //   this.viewForm.flowInstanceId = this.$refs.generateForm.getValue("probation_employee_approval_form_id")
      // }

      // this.viewForm.visible = true
      console.log(this.viewForm,'this.viewFrom+++')
    },
    viewRequestApprovalForm(data){
      const requestRow = data?.argument?.row || {};
      const otherBizId = requestRow.expenseReimbursementId || requestRow.id || data?.expenseReimbursementId;
      const parmas = {
        flowName: '',
        useScope: 'invest',
        auditWayList: ['request_funds'],
        statusList: ['end'],
        initiator: 'all',
      };
      if(otherBizId){
        parmas.flowInstanceBizRelevanceList = [
          {
            otherBiz: 'request_funds',
            otherBizId
          }
        ]
      }else if(data?.id){
        parmas.id = data.id;
      }else{
        return this.$message.warning('未找到关联请款单，无法查看流程详情');
      }
      this.$axios.post(
        Api.schedule.getFlowInstanceList,
        {
          data: parmas,
          pagination: true,
          pages: 1,
          size: 99
        },
        res => {
          if (res.isSuccess) {
            const flow = res.data && res.data[0];
            if(!flow){
              return this.$message.warning('未找到关联请款单流程，无法查看流程详情');
            }
            this.viewForm.formId = flow.formProxyId
            this.viewForm.flowInstanceId = flow.id;
            this.viewForm.visible = true
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    updateList(){
      this.$emit('success');
    },
    postExamine() {
      // this.bindBatchFileByIds();
      this.$emit('update:visible', false);
      this.$emit('success');
    },
    // 遍历对象的key获取匹配字符的值
    getValuesByKeyMatch(obj, searchString) {
      const matchedValues = {};
      for (const key in obj) {
        if (key.includes(searchString)) {
          matchedValues[key] = obj[key];
        }
      }
      return matchedValues;
    },
    // 根据用户获取隐藏的字段
    getUserHideField(){
      return new Promise((resolve, reject) => {
        this.$axios.post(
          '/web/flowProxy/getUserHideField',
          {
            data: {
            },
            flowInstanceId: this.flowInstanceId,
            batchNo: this.batchNo
          },
          (res) => {
            if (res.isSuccess) {
              const { data } = res;
              resolve(data)
            }
          }
        );
      });
    },
    // 获取表单模板
    async getFormDetail() {
      console.log('this.flowStatus',this.flowStatus)
      console.log('this.formId',this.formId)
      console.log('=========getFormDetail1=========',this.flowStatus)
      if (!this.formId) { // 无表单
        this.disabledData = [];
        this.jsonData = {
          config: {},
          list: []
        };
        return;
      }
      console.log(123)
      const editData = this.models = await this.getFormData();
      editData.flowInstanceId = this.flowInstanceId;
      console.log('editData111111111',editData)
      if (editData.dynamicParam) { // 判断是否是转发流程(有表单)
        console.log('是转发数据')
        this.isTranspondFlow = true;
        this.transpondFlowData = editData.dynamicParam;
        // 转发后的流程，为了沿用之前的流程逻辑，有些数据需要获取转发之前的数据
        this.selectFlowType = editData.transpondFlowInstance.selectFlowType;
        this.businessId = editData.transpondFlowInstance.businessId;
      }
      if(this.$refs.attachList){
        if(this.flowStatus=='draft'||this.flowStatus=='withdraw'){
          editData?.fileupload_common_file?.map(item=>{
            item.flowStatus = this.flowStatus
          })
        }
        this.$refs.attachList.setModels(editData.fileupload_common_file)
      }
      // 重新发起时，员工年度绩效考核表单的特定字段重置为0
      if (this.isReInitiate && this.selectFlowType === 'staff_annual_performance') {
         const resetFields = ['strategicUnderstanding', 'strategyAndMethod', 'annualAchievement', 'nextYearPlan', 'teamDevelopment', 'presentationSkill','finalScore'];
         resetFields.forEach(key => {
            editData[key] = undefined;
         });
      }
      console.log('测试审批意见-editData',editData,this.models)
      for (const item in editData) {
        // 审批意见
        // 第一种做法：
        let val = '';
        if(item.includes('auto_audit_info_') && !item.includes('auto_audit_info_obj_')){ // 后端返回的数据有'auto_audit_info_'，不一定有'auto_audit_info_obj_'
          // console.log('item',item)
          const number = item.split('auto_audit_info_')[1];
          // console.log('number',number)
          const key ='auto_audit_info_' + number;
          const key2 ='auto_audit_info_obj_' + number;
          const key3 ='auto_audit_info_obj_list_' + number;
          const rawAutoAuditInfo = this.formatAutoAuditInfoText(editData[item]);
          let hasObjKey = this.doesKeyContainChar(editData,key2); // 不能用includes，因为可能会误判断
          let hasObjList = Array.isArray(editData[key3]) && editData[key3].length > 0;
          if (hasObjKey || hasObjList) { // 有'auto_audit_info_'并且有对象数据
            if(this.selectFlowType =='employee_retirement_notice'){ // 员工退休通知单
              // 如果要使用到list数据集，打开注释
              if (editData[key3] && editData[key3].length > 1) {
                editData[key3].forEach(x=>{
                  val += `${x.auditName}<br>`
                })
              } else {
                let {auditDesc} = this.getDevice(editData[key2]?.auditDesc)
                if(!editData[key2])editData[key2] = {}
                editData[key2].auditDesc = auditDesc
                if(editData[key2].auditDesc)val = `${editData[key2].auditDesc||''}【${editData[key2].auditStatus}】\n${editData[key2].auditName} ${editData[key2].auditDate}`
                // val = `${editData[item].auditDesc||''}${editData[item].auditName}${editData[item].auditStatus} ${editData[item].auditDate}`
              }
              // val = `${editData[key2].auditName}`
            }else{
              console.log(1111111)
              console.log('editData[key3]',editData[key3])
              // 如果要使用到list数据集，打开注释
              if (editData[key3] && editData[key3].length > 1) {
                console.log('key3',key3)
                const auditInfoList = editData[key3].map(x => {
                  return this.formatAutoAuditInfoLine(x)
                }).filter(Boolean)
                val = auditInfoList.join('\n') || rawAutoAuditInfo
              } else {
                const auditInfo = editData[key2] || (hasObjList ? editData[key3][0] : null);
                if (auditInfo && auditInfo.auditName){
                  val = this.formatAutoAuditInfoLine(auditInfo)
                } else {
                  val = rawAutoAuditInfo
                }
              }
              // if (editData[key2].auditDesc){
              //   val = `${editData[key2].auditDesc||''}\n【${editData[key2].auditStatus}】\n${editData[key2].auditName} ${editData[key2].auditDate}`
              // } else {
              //   val = `${editData[key2].auditDesc||''}【${editData[key2].auditStatus}】\n${editData[key2].auditName} ${editData[key2].auditDate}`
              // }
            }
          } else { // 有'auto_audit_info_'没有'auto_audit_info_obj_'
            val = rawAutoAuditInfo || editData[item];
          }

          if(this.flowStatus =='draft'||this.flowStatus=='withdraw'){
            this.$set(this.editData, key, '');
          }else{
            console.log('val',val)
            this.$set(this.editData, key, val);
            console.log('this.editData',this.editData)
          }
        } else if (item.includes('auto_audit_info_obj_')) {
          this.$set(this.editData, item, editData[item]);
        } else if (!item.includes('auto_audit_info_') && !item.includes('auto_audit_info_obj_')) { // 不是审批意见的字段的赋值
          this.$set(this.editData, item, editData[item]);
        }
      }
      this.editData = Object.assign(editData,this.editData);
      // this.editData = Object.assign(this.editData,editData);
      console.log(33333333,this.editData)
      // const enableData = this.enableData = this.flowNodeProxyId ? await this.findApprovePermission() : [];
      const allPowerFieldData = this.flowNodeProxyId ? await this.findApprovePermission() : [];
      const enableData = this.enableData = allPowerFieldData.enableData;
      const hideData = allPowerFieldData.hideData;// 隐藏字段注释

      // console.log('enableData1',enableData)
      console.log('hideData1',hideData)
      this.$axios.post(
        Api.qualityManage.getTaskFormDetail,
        {
          data: {
            id: this.formId
          }
        },
        async (res) => {
          if (res.isSuccess) {
            let templateData = editData.formProxyData ? editData.formProxyData.templateData : res.data.templateData;
            // let copyTemplateData = JSON.parse(res.data.templateData);
            let copyTemplateData = JSON.parse(templateData);
            this.setRequireByPermission(copyTemplateData.list);
            this.setAutoAuditInfoFieldStyle(copyTemplateData.list);
            this.jsonData = JSON.parse(JSON.stringify(copyTemplateData));
            const fieldsTemplateList = editData.formProxyData ? editData.formProxyData.fieldsTemplateList : res.data.fieldsTemplateList;
            // const fieldsTemplateList = res.data.fieldsTemplateList;
            const disabledData = fieldsTemplateList.map(item => {
              return (item.englishName ?? '').replaceAll('_$$_', '.');
            });
            if(this.selectFlowType=='assets_transfer'){ //移交单
              let index = disabledData.indexOf('view_button')
              if(index > -1)disabledData.splice(index,1)
            }
            this.disabledData = disabledData;
            this.$nextTick(async (x) => {
              this.$refs.generateForm.refresh();
              if(this.selectFlowType=='travel_expense'){ // 差旅报销单设置费用明细id
                // this.getDetial()
              }

              this.$refs.generateForm.disabled(disabledData, true);

              this.$refs.generateForm.setData(this.editData); // 加多一步设置表单数据，是为了触发表单里面一些组件的赋值，生成虚拟字段。
              // this.$refs.generateForm.hide(hideData);// 隐藏字段注释

              var values = this.$refs.generateForm.getValues();
              let allValues= JSON.parse(JSON.stringify(values));
              // console.log('allValues-findApprovePermission',allValues)
              this.isFieldNotHide(allValues,hideData,enableData)

            if(this.selectFlowType  == 'travel_expense' && disabledData.indexOf('departmentId_col') > -1){
              let dateTime = this.editData.dateTime
              let year = dateTime.substr(0,4)
              if(year == '2025'){
                this.editData?.expenseBudgetList.forEach(async (el,index)=>{
                let departmentId = el.departmentId
                let id = departmentId[departmentId.length-1]
                let r = await this.getCostTypeById(id)
                  // document.querySelectorAll('[data-id="departmentId"]')[index].querySelector('.el-input__inner').value= `${r.departmentName}/${r.name}`
                  document.querySelectorAll('[data-id="departmentId"]')[index].innerHTML = `${r.departmentName}/${r.name}`//replaceWith(document.createTextNode(`${r.departmentName}/${r.name}`))
                })
              }
            }
            });
          }
        }
      );
    },
    getCostTypeById(id){
      return new Promise((resolve,reject)=>{
        this.$axios.post(Api.budgetManage.getCostTypeById,{data:{id}}).then(res=>{
          if(res.isSuccess){
            // resolve('res.data',res.data)
            resolve(res?.data)
          }else{
            resolve('')
          }
        })
      })

    },
    // 前提：流程设置了发起人的评分字段被隐藏，审核人没有隐藏（流程节点只有两个，一个是发起人，一个是审核人）
    isFieldNotHide(vals,hideData,enableData) { // 隐藏字段的特殊处理（中高层及关键岗位360评分表、员工360评分表的审批人是发起人，就不隐藏任何字段）
      console.log('isFieldNotHide')
      console.log('vals.additionalData',vals.additionalData)
      console.log('initiatorId', this.initiatorId)
      let id = localstorageGet('userId');
      this.isScoreFlag = false;
      if (this.isExamine && !this.isReInitiate) { // 审核阶段
        if(this.selectFlowType == 'year_kpi_report_scoring' && (id == vals.additionalData.initiatorUserId || id == this.initiatorId)) {
          this.isScoreFlag = true;
        }
      } else if (!this.isExamine && !this.isReInitiate) { // 查看详情
        let finalCheckId = this.checkFlowNode[this.checkFlowNode.length-1]['executorId']
        if (this.selectFlowType == 'year_kpi_report_scoring' && id == finalCheckId) {
          this.isScoreFlag = true;
        } else {
          this.isScoreFlag = false;
        }
      }
      // console.log('this.isScoreFlag',this.isScoreFlag)
      this.isScoreFlag || this.$refs.generateForm.hide(hideData);// 隐藏字段注释
      if (this.btnVisible) {
        this.$refs.generateForm.disabled(enableData, false);
        this.isScoreFlag || this.$refs.generateForm.hide(hideData);// 隐藏字段注释
      }
    },
    getAuditRecord() {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.approveManage.findRecord,
          {
            data: {
              flowInstanceId: this.flowInstanceId
            }
          },
          res => {
            if (res.isSuccess) {
              resolve(res.data);
            } else {
              this.$message.error(res.message);
            }
          }
        );
      });
    },
    // 前提：流程设置了发起人的评分字段被隐藏，审核人没有隐藏（流程节点只有两个，一个是发起人，一个是审核人）
    isFieldNotHide(vals,hideData,enableData) { // 隐藏字段的特殊处理（中高层及关键岗位360评分表、员工360评分表的审批人是发起人，就不隐藏任何字段）
      console.log('isFieldNotHide')
      let id = localstorageGet('userId');
      this.isScoreFlag = false;
      if (this.isExamine && !this.isReInitiate) { // 审核阶段
        if(this.selectFlowType == 'year_kpi_report_scoring' && id == vals.additionalData.initiatorUserId) {
          this.isScoreFlag = true;
        }
      } else if (!this.isExamine && !this.isReInitiate) { // 查看详情
        let finalCheckId = this.checkFlowNode[this.checkFlowNode.length-1]['executorId']
        if (this.selectFlowType == 'year_kpi_report_scoring' && id == finalCheckId) {
          this.isScoreFlag = true;
        } else {
          this.isScoreFlag = false;
        }
      }
      // console.log('this.isScoreFlag',this.isScoreFlag)
      this.isScoreFlag || this.$refs.generateForm.hide(hideData);// 隐藏字段注释
      if (this.btnVisible) {
        this.$refs.generateForm.disabled(enableData, false);
        this.isScoreFlag || this.$refs.generateForm.hide(hideData);// 隐藏字段注释
      }
    },
    getAuditRecord() {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.approveManage.findRecord,
          {
            data: {
              flowInstanceId: this.flowInstanceId
            }
          },
          res => {
            if (res.isSuccess) {
              resolve(res.data);
            } else {
              this.$message.error(res.message);
            }
          }
        );
      });
    },
    getDevice(val){
      let reg = /\#\{(.+?)\}\#/g
      let auditDesc,device='web'
      if(val){
        let find = val.match(reg)
        if(find && find.length){
          auditDesc = val.replace(find[0],'')
          device = find[0].replace('#{', '').replace('}#', '')
        }else{
          auditDesc = val
        }
      }
      return {auditDesc,device}
    },
    formatAutoAuditInfoLine(auditInfo = {}){
      let {auditDesc} = this.getDevice(auditInfo?.auditDesc);
      const opinion = (auditDesc || '').trim();
      const auditStatus = (auditInfo?.auditStatus || '').trim();
      const auditName = (auditInfo?.auditName || '').trim();
      const auditDate = (auditInfo?.auditDate || '').trim();
      const statusText = auditStatus ? `【${auditStatus}】` : '';
      return `${opinion ? `${opinion} ` : ''}${statusText}${statusText && (auditName || auditDate) ? ' ' : ''}${auditName}${auditName && auditDate ? ' ' : ''}${auditDate}`.trim();
    },
    formatAutoAuditInfoText(val){
      if (!val || typeof val !== 'string') {
        return '';
      }
      const lines = val.split(/\r?\n/).map(item => {
        let {auditDesc} = this.getDevice(item);
        return (auditDesc || '').trim();
      }).filter(Boolean);
      if (!lines.length) {
        return '';
      }
      const result = [];
      for (let i = 0; i < lines.length; i++) {
        const current = lines[i];
        const next = lines[i + 1];
        const third = lines[i + 2];
        if (current.startsWith('【')) {
          result.push(`${current}${next ? ` ${next}` : ''}`.trim());
          if (next) {
            i += 1;
          }
          continue;
        }
        if (next && next.startsWith('【')) {
          let text = `${current} ${next}`;
          if (third && !third.startsWith('【')) {
            text += ` ${third}`;
            i += 2;
          } else {
            i += 1;
          }
          result.push(text.trim());
          continue;
        }
        result.push(current);
      }
      return result.join('\n');
    },
    // 判断对象的key有无指定字符
    doesKeyContainChar(obj, charToFind) {
      for (const key in obj) {
        // if (key.includes(charToFind)) {
        if (key == charToFind) {
          return true; // 找到包含字符的键
        }
      }
      return false; // 没有找到包含字符的键
    },
    getDetial(){
      this.$refs.generateForm.sendRequest('获取流程数据明细',this.businessId).then(res=>{
        console.log(res,'获取表单详情！')
        const  expenseDetailList = []
        if(res.data&&res.data.expenseDetailList){
          res.data.expenseDetailList.map(item=>{
            expenseDetailList.push({type:item.type,money:item.money,id:item.id,remark:item.remark,fileupload_7z2cx2gc:[]})
          })
          this.$refs.generateForm.getValue('expenseDetailList').map(item=>{
            expenseDetailList.map(k=>{
              if(k.type==item.type&&k.money==item.money){
                k.fileupload_7z2cx2gc = item.fileupload_7z2cx2gc
              }
            })
          })
          console.log(expenseDetailList,'expenseDetailList+++')
          this.$refs.generateForm.setData({expenseDetailList}).then(r=>{
            console.log(res,'设置成功！')
          })
        }
      })
    },
    // 递归表单，根据配置的表单权限，修改表单是否需要校验配置（如果某个表单字段本身设置必填，但是没有配置权限，那么会调整为非必填；如果本身未设置必填，就算有权限，也不需要必填）
    setRequireByPermission(genList) {
      // console.log('this.enableData',this.enableData)
      // 1、子表单暂时未考虑子表单+这种容器的情况
      genList.map((item, key) => {
        if (item.type == 'table') { // 子表单容器（不支持布局嵌套）
          if (item.tableColumns) {
            // console.log('item.tip',item.options.customClass.indexOf('subFormNoNeedPermission')>-1)
            if (item.options.customClass.indexOf('subFormNoNeedPermission') == -1) { // 子表单如果不需要控制每列权限，就要加这个样式
              item.tableColumns.forEach(y=>{
                if (!this.enableData?.includes(y.model + '_col')) { // _col后缀用于配合制作多一个表格用于控制子表单每列权限
                  this.$set(y.options,'required',false);
                  this.$set(y,'rules',[]);
                }
              })
            }
          }
        } else if (item.type == 'grid') {
          item.columns.map(val => {
            this.setRequireByPermission(val.list);
          });
        } else if (item.type == 'report') {
          item.rows.map(val => {
            val.columns.map(i => {
              this.setRequireByPermission(i.list);
            });
          });
        } else if (item.type == 'inline') {
          this.setRequireByPermission(item.list);
        } else {
          if (item.model) {
            if (!this.enableData?.includes(item.model.replaceAll('_$$_', '.'))) {
              // 下面两行代码足够覆盖所有场景，即用下面两行；不够用可把注释打开，根据不同场景进行配置。
              // if (item.options.validatorCheck) { // 自定义校验规则
              //   this.$set(item.options,'validatorCheck',false);
              // } else if (item.options.validatorCheck && item.options.required) { // 自定义校验规则还点了必填
              //   this.$set(item.options,'validatorCheck',false);
              // } else if (item.options.patternCheck && item.options.required) { // 正则校验
              //   this.$set(item.options,'patternCheck',false);
              // } else if (item.options.dataTypeCheck && item.options.required) { // 表单内置的校验规则
              //   this.$set(item.options,'dataTypeCheck',false);
              // }
              this.$set(item.options, 'required', false);
              this.$set(item, 'rules', []);
              // console.log('item2',item)
            }
          }
        }
      });
    },
    setAutoAuditInfoFieldStyle(genList) {
      genList.map((item) => {
        if (item.type == 'table') {
          if (item.tableColumns) {
            item.tableColumns.forEach(val => {
              this.setAutoAuditInfoFieldStyle(val.list || []);
            });
          }
        } else if (item.type == 'grid') {
          item.columns.map(val => {
            this.setAutoAuditInfoFieldStyle(val.list || []);
          });
        } else if (item.type == 'report') {
          item.rows.map(val => {
            val.columns.map(i => {
              this.setAutoAuditInfoFieldStyle(i.list || []);
            });
          });
        } else if (item.type == 'inline' || item.type == 'card') {
          this.setAutoAuditInfoFieldStyle(item.list || []);
        } else if ((item.type == 'tabs' || item.type == 'collapse') && item.tabs) {
          item.tabs.map(val => {
            this.setAutoAuditInfoFieldStyle(val.list || []);
          });
        } else if (item.model && item.model.includes('auto_audit_info_') && !item.model.includes('auto_audit_info_obj_')) {
          if (!item.options) {
            this.$set(item, 'options', {});
          }
          let classList = (item.options.customClass || '').split(' ').filter(Boolean);
          ['approvalOpinion', 'autoAuditInfoField'].forEach(className => {
            if (!classList.includes(className)) {
              classList.push(className);
            }
          });
          this.$set(item, 'type', 'textarea');
          this.$set(item.options, 'customClass', classList.join(' '));
          this.$set(item.options, 'autosize', true);
          this.$set(item.options, 'rows', Math.max(Number(item.options.rows) || 2, 2));
          this.$set(item.options, 'resize', 'none');
        }
      });
    },
    unique(arr) {
      return arr.filter((item, index, arr) => {
        return arr.indexOf(item) === index;
      })
    },
    // 表单字段权限
    async findApprovePermission() {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.qualityManage.findApprovePermission,
          {
            data: {

            },
            nodeProxyId: this.flowNodeProxyId
          },
          async (res) => {
            if (res.isSuccess) {
              let enableData = [],hideData = [];
              if (res.data && res.data.flowNodeFieldPowerTemplateList) {
                // enableData = res.data.flowNodeFieldPowerTemplateList.map(item => item.formFieldTemplateEnglishName);
                enableData = res.data.flowNodeFieldPowerTemplateList.filter(x=> x.fieldPower != 'hide').map(item => (item.formFieldTemplateEnglishName ?? '').replaceAll('_$$_', '.'));
                // const originHideData = res.data.flowNodeFieldPowerTemplateList.filter(x=> x.fieldPower == 'hide').map(item => item.formFieldTemplateEnglishName);
                const userHideFieldResult = await this.getUserHideField() || [];// 这个接口获取的是前面审批过的节点，并且设置了隐藏的字段
                console.log('userHideFieldResult',userHideFieldResult)
                const userHideField = userHideFieldResult.filter(x=> x.fieldPower == 'hide').map(item => item.formFieldsProxyVo.englishName);// 隐藏字段注释
                // 审批过的节点隐藏字段还需要加上详情接口返回的当前接口隐藏的字段(注意：查看流程的时候就不要用详情接口返回的隐藏字段)
                const hideDataResult = res.data.flowNodeFieldPowerTemplateList.filter(x=> x.fieldPower == 'hide').map(item => item.formFieldTemplateEnglishName);
                hideData = userHideField.concat(hideDataResult)
                hideData = this.isExamine || this.isReInitiate ? this.unique(hideData) : userHideField;
                // hideData = this.isExamine ? this.unique(hideData) : (this.isReInitiate ? originHideData : userHideField);
                // hideData = !this.isExamine && !this.isReInitiate ? this.unique(hideData) : userHideField;
                // console.log('===originHideData====',originHideData)
                console.log('===userHideField====',userHideField)
                console.log('===hideDataResult====',hideDataResult)
                console.log('====hideData====',hideData)
                // hideData = res.data.flowNodeFieldPowerTemplateList.filter(x=> x.fieldPower == 'hide').map(item => item.formFieldTemplateEnglishName);// 隐藏字段注释
              }
              // resolve(enableData);
              resolve({
                enableData,
                hideData// 隐藏字段注释
              });
            }
          }
        );
      });
    },
    // 获取表单字段值
    getFormData() {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.qualityManage.getFormData,
          {
            data: {
              id: this.flowInstanceId
            },
            // excludeFieldNames: ['auto_audit_info_']
          },
          (res) => {
            if (res.isSuccess) {
              console.log(res,'888888',this.businessId)
              resolve(res.data.data);
            }
          }
        );
      });
    },
    handleBeforeSubmit(type) {
      console.log('handleBeforeSubmit-type', type)
      this.$refs.approveForm.validate((valid) => {
        if (valid) {
          if (type == 'pass') {
            if (this.flowNodeType == 'run_node_choose') {
              if (!this.checkboxRersonGroup.length) {
                // 未选择节点
                this.getNodeList();
                this.nodeChooseVisible = true;
              } else {
                this.nextAuditorList = [];
                this.checkboxRersonGroup.map(item => {
                  this.nextAuditorList.push({
                    bizId: item
                  });
                });
                this.handleSubmit(type, true);
              }
            } else {
              this.handleSubmit(type, false);
            }
          } else {
            this.handleSubmit(type, false);
          }
        } else {
          this.$message.error('请填写审批意见');
          return false;
        }
      });
    },
    handleSubmit(type, flag) {
      console.log('handleSubmit-type', type)
      this.$refs.generateForm.getData().then(x => { // 24.12.25改为不用getData返回的value，使用getValues获取(原因要获取表单虚拟字段)。getData只做校验
        let value = this.$refs.generateForm.getValues();
        let param = {};
        if (type == 'no_pass') {
          param = {
            data: {
              jobTaskId: this.jobTaskId,
              auditRecord: {
                auditStatus: type,
                executeDesc: this.approveForm.approveMessage
              }
            },
            formDataMongoVo: {
              data: value
            }
          };
        } else {
          if (flag) {
            param = {
              data: {
                jobTaskId: this.jobTaskId,
                auditRecord: {
                  auditStatus: type,
                  executeDesc: this.approveForm.approveMessage
                }
              },
              nextAuditorList: this.nextAuditorList,
              formDataMongoVo: {
                data: value
              }
            };
          } else {
            param = {
              data: {
                jobTaskId: this.jobTaskId,
                auditRecord: {
                  auditStatus: type,
                  executeDesc: this.approveForm.approveMessage
                }
              },
              formDataMongoVo: {
                data: value
              }
            };
          }
        }
        this.$axios.post(
          Api.qualityManage.submitTask,
          param,
          (res) => {
            if (res.isSuccess) {
              this.$message.success('提交成功');
              this.$emit('update:visible', false);
              this.$emit('success');
            } else {
              this.$message.error(res.message);
            }
          }
        );
      }).catch(e => {
        console.log(e);
      });
    },
    handleClose() {
      this.approveForm.approveMessage = '';
      this.$emit('update:visible', false);
      this.models = {};
      this.editData = {};
    },
    // 获取节点人员列表
    getNodeList() {
      const nextNodeProxyId = this.nextNodeProxyId;
      this.$axios.post(
        Api.approveManage.getFlowNodeProxyConfigUserList,
        {
          data: {
            id: nextNodeProxyId
          }
        },
        res => {
          if (res.isSuccess) {
            this.personList = res.data.flowNodeAuditConfig.userVoList;
          }
        }
      );
    },
    // 选择节点人员
    handleCheckedPreson(val) {
    },
    // 取消选择节点人员
    handleCancelCheckNode() {
      this.nodeChooseVisible = false;
      this.checkboxRersonGroup = [];
    },
    // 获取审批节点人员
    handleGetNode() {
      this.nodeChooseVisible = false;
      this.handleBeforeSubmit('pass');
    },
    onFileUpload(obj) {
      this.uploading = obj;
    },
    onChange(field, val, models) {
      console.log('onChange', field, val)
      this.uploadedFile = { field, val };
    }
  }
};
</script>
<style>
.from_text_link{
color: #005fff;
cursor: pointer;
}
</style>


<style lang="scss" scoped>
.expandBar-btn {
  transform-origin: top left;
  transform: scale(0.6);
  background: #376eac;
  box-shadow: -1px 2px 5px 2px #ccc;
  width: 43px;
  height: 35px;
  padding-right: 3px;
  color: #fff;
  font-size: 24px;
  text-align: center;
  line-height: 31px;
  position: fixed;
  z-index: 3;
  cursor: pointer;
  top: 50%;
}
.flex-right{
  border-radius: 5px 50px 50px 5px;
}
.flex-left{
  right: 10px;
  border-radius: 50px 5px 5px 50px;
}
@page {
  size: auto !important; // 控制打印纵向和横向选择
  margin: 3mm;
  margin-bottom: 5mm;
  margin-top: 5mm;
}
// formMaking表单打印样式
@media print {
  // html {
  //   zoom:10%;
      // margin: 0px !important;
      // height: auto !important;
  // }
  // body {
  //     border: solid 1px white !important;
  //     margin: 10mm 15mm 10mm 15mm !important;
  // }
  #formContainer {
    zoom: 0.8;
    // zoom:98%;
    margin-top: 40px;
    // page-break-inside: avoid;
    // 流程样式
    ::v-deep {
      .postscript-divWrap { // 附言样式
        margin: 0 auto;
        padding: 0px 50px;
        .flow-wrap {
          width: 100% !important;
          padding: 0px;
          // width: 900px !important;
          .postscript-list {
            max-height: max-content !important;
          }
        }
      }
      .flow-log-container { // 流程日志样式
        margin: 0 auto;
        overflow: initial;
        padding: 0px 50px;
        .flowWrap {
          margin-top: 0px !important;
          padding: 0px;
        }
      }
      .transpondFlow-wrap {
        padding: 10px 24px !important;
      }
    }

    // 表单打印样式
    ::v-deep {
      .fm-report-table__table tr:has(.printNoShow){ // 隐藏表格整行
        display: none !important;
      }
      .el-form .fm-form-item:has(.addHiddenBtn){ // 子表单因为按钮隐藏，导致顶部边框消失
        display: none !important;
      }
      // 子表单样式调整
      .form-subform .form-subform-item .form-subform-index {
        display: none;
      }
      .el-table th.el-table__cell.required > div.required::before {
        display: none;
      }
      .el-form-item.noPrintFile .fm-upload-file .upload-list { // 上传的文件不打印
        display: none;
      }
      .el-form-item.noPrint { // 普通隐藏某一块内容
        display: none;
      }
      .el-form-item.is-error .el-form-item__error { // 隐藏错误提示
        display: none;
      }
      // 11.21修改打印字体大小
      .fm-form .print-read-label{
        font-size: 16px !important;
      }
      .fm-form .title .print-read-label{ // 表单标题样式
        font-weight: 700;
        font-size: 28px !important;
        line-height: 60px !important;
      }
      // .fm-form .el-form-item .el-form-item__label {
      //   font-size: 16px !important;
      // }
      .fm-form table{
        font-size: 16px !important;
      }
      // 下面是针对差旅报销单里面费用明细子表单最后一列的文件列，打印缩小宽度
      .el-form-item.lastFileColumn .el-table .el-table__header-wrapper colgroup col:nth-last-child(2) { // 某个子表单最后一列文件打印需要宽度调整
        width: 120px !important;
      }
      .el-form-item.lastFileColumn .el-table .el-table__body-wrapper colgroup col:last-child { // 某个子表单最后一列文件打印需要宽度调整
        width: 120px !important;
      }
      .el-form-item.lastFileColumn .el-table .el-table__fixed .el-table__fixed-header-wrapper colgroup col:last-child { // 某个子表单最后一列文件打印需要宽度调整
        width: 120px !important;
      }
      .el-form-item.lastFileColumn .el-table .el-table__fixed .el-table__fixed-body-wrapper colgroup col:last-child { // 某个子表单最后一列文件打印需要宽度调整
        width: 120px !important;
      }
      // 子表单删除按钮不打印，避免打印出来有删除按钮，影响美观
      .fm-form .form-subform-remove {
        display: none !important;
      }

      // .el-form-item.approvalOpinion .el-form-item__content {
      //   white-space: pre-wrap;
      //   font-size: 20px;
      // }
      // .print-read-label {
      //   font-size: 20px;
      //   font-weight: 400;
      //   color: #606266;
      // }
      // .fm-form-item:has(.noPrintTitle) { // 表单头部不打印
      //   visibility: hidden;
      //   // display: none;
      // }


      // 老版本开始
      // .el-table__fixed-right {
      //   border-left: 2px solid rgb(153, 153, 153);
      // }
      // .addHiddenBtn { // 自定义的需要隐藏的按钮
      //   display: none;
      // }
      // .addLeftBtn {
      //   display: block !important;
      // }
      // .el-table__header-wrapper thead.has-gutter{
      //   border-bottom: 2px solid rgb(153, 153, 153);
      // }

    }

    ::v-deep {
      .newFormMaking { // 打印样式
        // 2024.10.31表单整体样式优化
        .el-form-item.tableAddBorder .form-table {
          border: none; // 2025.1.8更换新打印插件后更新的样式
          // border-color: rgb(153, 153, 153);
          // border: 1px solid rgb(153, 153, 153); // 之前用的
        }
        .el-table th.el-table__cell.is-leaf,.el-table td.el-table__cell {
          border-color: rgb(153, 153, 153);
        }
        .el-form-item.tableNoPadding{ // 这个是否要，看看其他表单
          border: 1px solid rgb(153, 153, 153);
        }
        .el-form-item.approvalOpinion .el-form-item__content {
          white-space: pre-wrap;
          font-size: 20px;
        }
        .el-form-item.autoAuditInfoField {
          margin-bottom: 0 !important;
        }
        .el-form-item.autoAuditInfoField .el-form-item__content {
          display: block;
        }
        .el-form-item.autoAuditInfoField .el-textarea__inner {
          min-height: 56px !important;
          line-height: 1.5;
          white-space: pre-wrap;
          word-break: break-all;
          overflow: hidden !important;
        }
        .print-read-label {
          // font-size: 20px !important; // 2025.1.2注释掉
          // font-weight: 400; // 2025.1.2注释掉
          color: #606266;
        }
        .el-form-item.titleBold .print-read-label,.el-form-item.title .print-read-label {
          font-size: 28px !important;
          font-weight: 700 !important;
          color: rgb(51, 51, 51);
        }

        // .el-form-item.approvalOpinion .el-form-item__content {
        //   white-space: pre-wrap;
        //   font-size: 16px;
        // }
        // .print-read-label {
        //   font-size: 16px;
        //   font-weight: 400;
        //   color: #606266;
        // }
        .el-form-item.tableNoPadding{
          padding: 0px;
        }

        // 11.21修改打印字体大小
        .el-form-item .el-form-item__content, .el-form-item .el-form-item__label{
          font-size: 18px !important;
        }
        .el-form-item.sub-title .el-form-item__label {
          font-size: 28px !important;
          font-weight: 700;
        }
        .el-form-item.title .el-form-item__label {
          font-size: 28px !important;
          font-weight: 700;
        }

        // .el-form-item.titleBold .el-input__inner { // 标题是选公司的组件样式（11.28）
        //   font-size: 28px !important;
        //   font-weight: 700 !important;
        //   height: 40px;
        //   line-height: 40px;
        // }

        .el-table .el-table__header-wrapper thead th .cell{ // 25.1.8添加表格头字体大小
          text-align: center !important;
          color: #000 !important;
          font-size: 18px !important;
        }
      }
    }
  }
}

::v-deep {
  .fm-form {
    // 禁用状态的字体颜色优化
    .el-checkbox__input.is-disabled + span.el-checkbox__label,.el-radio__input.is-disabled + span.el-radio__label,.el-range-editor.is-disabled input,.el-range-editor.is-disabled .el-range-separator{
      color: #606266 !important;
    }
    .el-checkbox__input.is-disabled.is-checked .el-checkbox__inner::after {
      border-color: #606266 !important;
    }
    .el-radio__input.is-disabled.is-checked .el-radio__inner::after{
      background-color: #606266 !important;
    }
    .newFormMaking {
      // 2024.10.31表单整体样式优化
      .el-form-item.tableAddBorder .form-table {
        border: 1px solid rgb(153, 153, 153);
      }
      .el-table th.el-table__cell.is-leaf,.el-table td.el-table__cell {
        border-color: rgb(153, 153, 153);
      }
      .el-form-item.is-error {
        margin-bottom: 10px !important;
      }
      .el-form-item.approvalOpinion .el-form-item__content {
        white-space: pre-wrap;
        font-size: 16px;
      }
      .el-form-item.autoAuditInfoField {
        margin-bottom: 0 !important;
      }
      .el-form-item.autoAuditInfoField .el-form-item__content {
        display: block;
      }
      .el-form-item.autoAuditInfoField .el-textarea__inner {
        min-height: 56px !important;
        line-height: 1.5;
        white-space: pre-wrap;
        word-break: break-all;
        overflow: hidden !important;
      }

      .el-table .el-table__body-wrapper tbody tr td:last-child { //11.7新增
        border-right: none;
      }
      .el-table .el-table__header-wrapper thead tr th:nth-last-child(2) { //11.7新增
        border-right: none;
      }

      // .el-form-item { // 11.7从formMaking的css剥离 （2025.1.2注释，好像有这个样式会导致表单数据多了，子表单数据打印不显示的问题）
        //   padding: 3px;
      // }
      .el-form-item.tableNoPadding{ // 11.7从formMaking的css剥离
        padding: 0px;
      }

      .el-table .el-table__header-wrapper th { // 2025.8.20加的样式，匹配子表单的合并表头
        border-bottom: 1px solid rgb(153, 153, 153);
      }

      .el-form-item.titleBold .el-input__inner { // 标题是选公司的组件样式（11.28）
        font-size: 28px !important;
        font-weight: 700 !important;
        height: 40px;
        line-height: 40px;
        border: none;
      }
      .el-form-item.title .el-input__inner {
        border: none;
      }
      // .el-table--border .el-table__body-wrapper .cell, .el-table--border .el-table__fixed-right .cell
      .el-table .el-table__header-wrapper thead th .cell{ // 11.29添加子表单表哥头居中
        text-align: center !important;
        color: #000 !important;
        font-size: 16px !important;
      }

      .fm-form-item:has(.title){ // 表格头标题对齐
        vertical-align: middle !important;
      }
      .fm-form-item .el-form-item.title .el-date-editor .el-input__inner,
      .fm-form-item .el-form-item.title .el-input-number .el-input__inner,
      .fm-form-item .el-form-item.title .el-select .el-input__inner,
      .fm-form-item .el-form-item.title .custom-container .el-textarea__inner,
      .fm-form-item .el-form-item.title .custom-container .el-input__inner { // 表格头标题对齐（其他控件，非文字控件） { // 表格头标题对齐
        font-weight: 700;
        font-size: 28px !important;
        text-align:center;
        height: 36px;
      }

      .btnCenter .fm-upload-file { // 按钮居中（文件上传）
        text-align: center;
      }

      // 兼容老版本
      .form-table .el-table--border {
        border: none !important;
      }

      .fm-form-item:has(.title),.fm-form-item:has(.sub-title){
        margin-bottom: 10px !important;
      }

      // 子表单+按钮样式
      .form-subform-item .form-subform-index .el-tag {
        margin-top: 10px;
      }
      .form-subform-item.is-hover .form-subform-remove .el-popover__reference {
        margin-top: 10px;
      }

      // 文字组件必填红点样式
      .el-form-item.showRedPot .el-form-item__label::before {
        content: "*";
        color: red;
        margin-right: 5px;
      }
    }
  }
}

::v-deep .dialog-fullscreen.isExamine .el-dialog__body {
  max-height: 66vh !important;
  min-height: 66vh !important;
  overflow-y: auto;
}

.dialog-container {
  .el-tabs__content {
    height: 710px;
    overflow: auto;
  }
}

.steps-container {
  margin-top: 20px;
  height: 100px;

  &.pass {
    .left-container {
      background: rgb(53, 219, 160);
    }

    .right-container {
      color: rgb(53, 219, 160) !important;
      border: 1px solid rgb(53, 219, 160);
    }
  }

  &.reject {
    .left-container {
      background: rgb(255, 64, 64);
      color: #fff;
    }

    .right-container {
      border: 1px solid rgb(255, 64, 64);
      color: rgb(255, 64, 64) !important;
    }
  }

  .left-container {
    width: 40px;
    height: 100%;
    line-height: 100px;
    color: #fff;
    background: rgb(187, 187, 187);
  }

  .right-container {
    height: 100%;
    padding: 10px;
    box-sizing: border-box;
    color: #303133 !important;
    border: 1px solid rgb(187, 187, 187);
    text-align: left;
  }
}



::v-deep {
  .fm-form {
    margin: 0 auto;
  }
  .fm-form .el-table--border .el-table__cell:first-child .cell {
    padding-left: 0px;
  }
  .fm-form .fm-report-table__table .el-table__cell .cell,.fm-form .form-table .el-table__cell .cell { // 子表单和表格单元格间距
    padding: 0;
  }
  .fm-form .fm-report-table__table .el-table__cell,.fm-form .form-table .el-table__cell { // 子表单和表格单元格间距
    padding: 0;
  }
  .fm-form .el-table th.el-table__cell.required>div.required::before{
    content: '*';
    color: #f56c6c;
    margin-right: 4px;
    background: transparent;
    vertical-align: top;
  }
  .fm-form .el-textarea__inner{
    overflow:auto;
  }
  .fm-form .hasAutosize.el-textarea .el-textarea__inner{
    overflow: hidden !important;
    padding: 2px !important;
  }
  .exame-form{
    .el-form-item .el-form-item__label{
      color: rgb(51, 51, 51);
    }
    .el-form-item .el-input .el-input__inner{
      color: rgb(51, 51, 51);
    }
    .form-table .el-table__fixed-header-wrapper div,
    .form-table .el-table__fixed-header-wrapper th,
    .form-table .el-table__header-wrapper,
    .form-table .el-table__header-wrapper div,
    .form-table .el-table__header-wrapper th{
      background: transparent;
    }
    .el-table thead{
      color: rgb(51, 51, 51);
      font-weight: normal !important;
    }
    .el-radio .el-radio__label{
      color: rgb(51, 51, 51);
    }
    .fm-form .print-read-label {
      font-size: 16px;
      font-weight: 400;
      color: #606266;
    }
  }

  .el-form-item--small .el-form-item__error {
    z-index: 10 !important;
  }

  .el-form-item__error {
    z-index: 10 !important;
  }
  .el-input.onlyRead .el-input__inner {
    border: none;
    cursor: not-allowed;
  }


  .fm-form .el-table th.el-table__cell.is-leaf,.fm-form .el-table td.el-table__cell { // 新版本分享
    border-color: rgb(153, 153, 153);
  }

  // 老版本
  .fm-form .form-table .el-table--border {
    border: 1px solid rgb(153, 153, 153) !important;
  }

  // 自定义的通用信息组件，多行文本不用显示滚动条
  .fm-form .custom-container .info-textarea .el-textarea__inner {
    overflow: hidden !important;
    padding: 2px !important;
  }
}

::v-deep {
  .el-dialog__body {
    height: calc(100% - 30px);
    max-height: initial !important;
    min-height: initial !important;

    .examine-content {
      display: flex;
      height: 100%;
      justify-content: center;
      overflow: hidden;

      .left-side {
        // width: 80%;
        display: flex;
        flex-direction: column;
        flex: 1;
        min-width: 500px;
        padding: 0px;
        // padding: 10px 0;
        overflow-y: auto;
        overflow-x: hidden;
        // overflow: auto;

        .container {
          height: 100% !important;
        }
      }

      .right-side-wraper {
        min-width: 280px;
        padding: 10px 0;
        margin-left: 10px;
        display: flex;
        flex-direction: column;
        max-width: 20%;

      }
      .right-side-wraper.noShowWrap {
        min-width: 10px;
        .right-side {
          display: none;
        }
      }
      .right-side {
        height: 100%;
        display: flex;
        flex-direction: column;
        // min-width: 280px;
        // padding: 10px 0;
        // margin-left: 10px;
        // display: flex;
        // flex-direction: column;
        // max-width: 20%;

        .flow-wrap {
          width: 100%;
          padding: 0;
        }
      }
    }
  }
}

@media screen and (max-width: 1440px) {
  ::v-deep {
    .el-dialog__body {
      .examine-content {
        .right-side-wraper {
          min-width: 200px;
          // overflow: auto;
        }
        .right-side-wraper.noShowWrap {
          min-width: 10px;
          .right-side {
            display: none;
          }
        }
        .right-side {
          height: 100%;
          display: flex;
          flex-direction: column;
          // min-width: 200px;
        }
        .left-side{
          overflow: auto;
        }
      }
    }
  }
}



// ::v-deep .fm-form .fm-report-table__table .fm-report-table__td {
//   padding: 3px !important;
// }
// rgb(102, 102, 102)

</style>
