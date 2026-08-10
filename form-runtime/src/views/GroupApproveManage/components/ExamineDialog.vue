<!--
 * @Descripttion: 查看、审核、表单弹窗（固定页面的审核弹窗）
 * @Author: zhengzetao
 * @Date: 2022-06-15
-->
<template>
  <div class="dialog-container" style="padding-bottom: 100px;">
    <template v-if="visible">
      <component v-bind:is="currentComponent" :visible.sync="visible" title="" :modal-append-to-body="true" :style="{zoom: detectZoom() > 140 ? 0.88 : 1}"
        :close-on-click-modal="false" :custom-class="isExamine ? 'isExamine' : ''
      " center @close="handleClose" :append-to-body="false" :before-close="handleClose" width="800px" v-bind="$attrs" :fullscreen="true" :modal="false">
        <div class="examine-content">
          <!-- 左侧侧边栏 -->
          <div class="left-side">
            <!-- 公共关联流程 -->
            <FormCommonFlowPost ref="flowListSelect" :flowInstanceBizRelevanceList="flowInstanceBizRelevanceList" :initiatorId="initiatorId"
            :isExamine="isExamine" :isReInitiate="isReInitiate" />
            <!-- 费用报销单 -->
            <ExpensesClaimForm :operaType="operaType" :id="flowId" :flowProxyId="flowProxyId"
              :flowNodeProxyId="flowNodeProxyId" :isExamine="isExamine" v-if="searchFlowType == 'expense_budget'" ref="ExpensesClaimForm"
               :flowInstanceId="flowInstanceId" :logTableData="logTableData" :isTranspondFlow="isTranspondFlow" :transpondFlowData="transpondFlowData">
            </ExpensesClaimForm>
            <!-- 金额调剂 -->
            <CompanyAmountAdjustForm :operaType="operaType" :id="flowId"
              v-if="searchFlowType == 'expense_reimbursement'" ref="CompanyAmountAdjustForm">
            </CompanyAmountAdjustForm>

            <!-- 公司年度预算 -->
            <DetailBudgetForm :params="params" :operaType="operaType" :id="flowId" v-if="searchFlowType == 'company_annual_budget' ||
      searchFlowType == 'add_annual_budget'" @updateParams="updateParams" @showLog="showLog"
              :flowNodeProxyId="flowNodeProxyId" :ref="searchFlowType" :isExamine="isExamine" :flowInstanceId="flowInstanceId" :flowProxyId="flowProxyId"
              :actionType="actionType" :createrId="initiatorId"
              >
            </DetailBudgetForm>

            <!-- 月度预算 -->
            <MonthBudgetForm :param="params" :operaType="operaType" :id="flowId"
              v-if="searchFlowType == 'depart_monthly_budget'" @updateParams="updateParams"
              :flowNodeProxyId="flowNodeProxyId" :ref="searchFlowType" :isExamine="isExamine" :flowInstanceId="flowInstanceId" :flowProxyId="flowProxyId"
              :actionType="actionType" showType="detail" :createrId="initiatorId"
              ></MonthBudgetForm>

            <!-- 项目预算 -->
            <ProjectBudgetForm :param="params" :operaType="operaType" :id="flowId" v-if="searchFlowType == 'project_setup_budget' ||
      searchFlowType == 'add_project_budget'" @updateParams="updateParams" @showLog="showLog"
              :flowNodeProxyId="flowNodeProxyId" :ref="searchFlowType" :isExamine="isExamine" :flowProxyId="flowProxyId">
            </ProjectBudgetForm>

            <!-- 年度绩效 一般人员目标责任书-->
            <template v-if="searchFlowType == 'annual_perf'||searchFlowType == 'year_kpi_work_target'">
              <workTarget :flowProxyId="flowProxyId" :isReInitiate="isReInitiate" :ref="searchFlowType" :searchFlowType="searchFlowType"
                v-if="flowType == 'workTarget'" :flowNodeProxyId="flowNodeProxyId" :actionType="actionType"
                :bizId="flowId" :flowInstanceId="flowInstanceId" :isTranspondFlow="isTranspondFlow">
              </workTarget>
              <ManagePerformence :flowProxyId="flowProxyId" :isReInitiate="isReInitiate" :ref="searchFlowType" v-else :searchFlowType="searchFlowType"
                :flowNodeProxyId="flowNodeProxyId" :actionType="actionType" :bizId="flowId" :flowInstanceId="flowInstanceId" :isTranspondFlow="isTranspondFlow">
              </ManagePerformence>
            </template>

            <!-- 月度绩效 -->
            <MonthlyPerformence :flowProxyId="flowProxyId" v-if="searchFlowType == 'monthly_perf'"
              :isReInitiate="isReInitiate" ref="monthly_perf" :flowNodeProxyId="flowNodeProxyId"
              :actionType="actionType" :bizId="flowId" :flowInstanceId="flowInstanceId" :isTranspondFlow="isTranspondFlow"></MonthlyPerformence>

            <!-- 月度绩效汇总 -->
            <monthPerfSum v-if="searchFlowType == 'monthly_perf_companySum' || searchFlowType == 'monthly_perf_departmentSum'"
            :flowProxyId="flowProxyId" :isReInitiate="isReInitiate" :ref="searchFlowType" :flowNodeProxyId="flowNodeProxyId" :selectFlowType="searchFlowType" :flowInstanceId="flowInstanceId"
              :actionType="actionType" :bizId="flowId"></monthPerfSum>

            <!-- 后备英才月度绩效汇总 -->
            <reserveMonthSum v-if="searchFlowType == 'reserveMonthSum'"
            :flowProxyId="flowProxyId" :isReInitiate="isReInitiate" :ref="searchFlowType" :flowNodeProxyId="flowNodeProxyId" :selectFlowType="searchFlowType" :flowInstanceId="flowInstanceId"
              :actionType="actionType" :bizId="flowId"></reserveMonthSum>

            <!-- 员工年终考核（无表单） -->
            <StaffAnnualAssessmentNoForm v-if="searchFlowType == 'staff_annual_assessment'"
              :flowProxyId="flowProxyId" :isReInitiate="isReInitiate" :ref="searchFlowType"
              :flowNodeProxyId="flowNodeProxyId" :actionType="actionType" :bizId="flowId"
              :flowInstanceId="flowInstanceId" :isExamine="isExamine" :logTableData="logTableData">
            </StaffAnnualAssessmentNoForm>

            <!-- 集团各公司年度预算流程 -->
            <GroupFinance :flowProxyId="flowProxyId" v-if="searchFlowType == 'group_finance'"
              :isReInitiate="isReInitiate" ref="group_finance" :flowNodeProxyId="flowNodeProxyId"
              :actionType="actionType" :bizId="flowId" :operaType="operaType" :id="flowId" :isExamine="isExamine">
            </GroupFinance>

            <!-- 公司月度预算汇总流程 -->
            <companyMonthlyFinance :flowProxyId="flowProxyId" v-if="searchFlowType == 'company_monthly_budget'"
              :isReInitiate="isReInitiate" ref="company_monthly_budget" :flowNodeProxyId="flowNodeProxyId"
              :actionType="actionType" :bizId="flowId" :operaType="operaType" :id="flowId" :isExamine="isExamine">
            </companyMonthlyFinance>

            <!-- 新建事项流程 -->
            <NewEvent v-if="copyTemplateDataComputed && searchFlowType == 'add_event_flow'" ref="add_event_flow" :type="'edit'"
            :isReInitiate="isReInitiate" :copyTemplateData="copyTemplateData" :flowProxyId="flowProxyId" :flowInstanceId="flowInstanceId"
            :initiatorId="initiatorId"></NewEvent>
            <!-- :isPrintPage="isPrintPage"  -->


            <!-- 图纸和文件审核 -->
            <template v-if="searchFlowType == 'DRAWINGS'">
              <div style="
              box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
              width: 950px;
              margin: auto;
              padding: 20px;
            ">
                <el-table :data="tableData" style="max-height: 300px; overflow: auto">
                  <el-table-column type="index" width="50"> </el-table-column>
                  <el-table-column prop="fileName" label="文件名称">
                    <template slot-scope="scope">
                      {{ scope.row.fileName || scope.row.name }}
                    </template>
                  </el-table-column>
                  <el-table-column label="操作" width="180">
                    <template slot-scope="scope">
                      <el-button size="mini" type="text" @click="viewFile(scope.row)">在线查看</el-button>
                    </template>
                  </el-table-column>
                </el-table>
              </div>
            </template>

            <!-- 转发的流程和附言 -->
            <TranspondLog :printStyle="true" v-if="isTranspondFlow" :flowInstanceId="flowInstanceId" :logTableData="logTableData" :transpondFlowData="transpondFlowData" :isNoEnterprise="false"></TranspondLog>
          </div>
          <!-- 左侧侧边栏 -->

          <!-- 左右收起栏 -->
          <div class="flex-right expandBar-btn" v-if="rightTabShow" @click="flexRight" :key="1">>></div>
          <div class="flex-left expandBar-btn" v-else @click="flexRight" :key="2">{{'<<'}}</div>

          <!-- 右侧侧边栏 -->
          <div class="right-side" v-show="rightTabShow">
            <el-button type="primary" icon='el-icon-view' @click="handleCheckFlow">查看流程</el-button>
            <!-- 发起人附言 -->
            <Postscript :flowInstanceId="flowInstanceId" v-bind="$attrs"></Postscript>
            <!-- 流程日志 -->
            <FlowLog :flowInstanceId="flowInstanceId" :logTableData="logTableData" :isNoEnterprise="false"></FlowLog>
            <eleUpload
            :style="!isExamine && 'margin-bottom:5px'"
            :showOnly="!isExamine"
            ref="eleupload"
            :size="20"
            :attachFile="attachFile"
          ></eleUpload>
            <!-- <span class="dialog-footer"> -->
            <!-- 审批意见 -->
            <ExamineOpinion v-if="visible" :isExamine="isExamine" :isReInitiate="isReInitiate"
              @postExamine="postExamine" :jobTaskId="jobTaskId" :flowNodeType="flowNodeType"
              :nextNodeProxyId="nextNodeProxyId" :nextNodeName="nextNodeName" :initiatorId="initiatorId"
              :searchFlowType="searchFlowType" :flowInstanceId="flowInstanceId" :id="flowId" :flowProxyId="flowProxyId"
              @beforeHandle="beforeHandle" v-bind="$attrs" v-on="$listeners" :flowName="flowName" :auditPassLogicFlag="auditPassLogicFlag"
              :isTranspondFlow="isTranspondFlow" @printPage="printPage" :formAndTranspondData="formAndTranspondData"
              :copyTemplateData="copyTemplateData" :formId="formId" :flowNodeProxyId="flowNodeProxyId" :outTracking.sync="outTracking" @updateList="updateList"
              :outFlowNodeTemplate="allFlowNodeDetail" :isTimeoutPage="isTimeoutPage" :hideTrackingButton="hideTrackingButton"
              :flowInstanceBizRelevanceList="flowInstanceBizRelevanceList" @bindFile="bindFile">
            </ExamineOpinion>
            <!-- </span> -->
          </div>
        </div>
      </component>
    </template>
    <CheckFlowNodeDetail v-if="checkViewFlowDetailVisible" :dialogVisible.sync="checkViewFlowDetailVisible"
              :flowInstanceId="flowInstanceId" :flowId="checkFlowId" :initiatorId="initiatorId" :companyId="companyId"></CheckFlowNodeDetail>
    <!-- <el-dialog :visible.sync="visible" title="" :modal-append-to-body="true" :close-on-click-modal="false"
      :custom-class="isExamine ? 'dialog-fullscreen isExamine' : 'dialog-fullscreen'" center @close="handleClose"
      :before-close="handleClose" width="800px" v-if="isDialog">

  </el-dialog> -->

    <!-- 审批节点选择：建立流程时如果选择了审批人自选，点击提交需要选下一节点审批人 -->
    <!-- <NextNodeDialog :visible.sync="nodeChooseVisible" :nextNodeName="nextNodeName" :nextNodeProxyId="nextNodeProxyId"
           @getSelectPerson="getSelectPerson" v-if="nodeChooseVisible">
         </NextNodeDialog> -->
  </div>
</template>

<script>
import eleUpload from '@/components/EleUpload';
import TranspondLog from './TranspondLog';
import ExpensesClaimForm from '@/views/BudgetManage/CompanyBudget/components/ExpensesClaimForm.vue';
import CompanyAmountAdjustForm from '../Submitted/components/Form/CompanyAmountAdjustForm.vue';
import DetailBudgetForm from '@/views/BudgetManage/CompanyBudget/components/detailBudget.vue';
import MonthBudgetForm from '@/views/BudgetManage/MonthlyBudget/components/AddMonthlyBudget.vue';
import ProjectBudgetForm from '@/views/BudgetManage/ProjectBudget/components/NewBudget.vue';
import workTarget from '@/views/PerformanceManage/TargetBook/WorkTarget/index';
import ManagePerformence from '@/views/PerformanceManage/TargetBook/ManageTarget/index';
import MonthlyPerformence from '@/views/PerformanceManage/MonthlyPerf/MonthlyPerf.vue';
import monthPerfSum from '@/views/PerformanceManage/monthlyPerfBlocSum/monthPerfSum.vue';
import reserveMonthSum from '@/views/PerformanceManage/reserveMonthSum/reserveMonthSum.vue';
import NewEvent from '@/views/BacklogManage/components/NewEvent.vue';
import ExamineOpinion from './examineOpinion';
import GroupFinance from '@/views/BudgetManage/CompanyBudget/components/groupFinance.vue';
import companyMonthlyFinance from '@/views/BudgetManage/CompanyBudget/components/companyMonthlyFinance.vue'
import Postscript from './Postscript.vue';
import FlowLog from './flowLog';
import StaffAnnualAssessmentNoForm from '@/views/PerformanceManage/staffAnnualAssessment/AnnualAssessmentNoForm.vue';
import Api from '@/api';
import mixin from './mixin';
import { viewFile, detectZoom } from '@/utils';
import CheckFlowNodeDetail from '@/views/GroupApproveManage/components/CheckFlowNodeDetail.vue';
import { localstorageGet } from '@/utils/auth';
import FormCommonFlowPost from '@/views/GroupApproveManage/components/FormCommonFlowPost/index.vue'
// import { importAll } from '@/utils';
// // 创建Context
// const allFile = importAll(
//   require.context('@/views/GroupApproveManage/Submitted/components/Form/', true, /\.vue$/)
// );
// // 生成待待注册组件集合
// const componentsAll = {};
// for (const key in allFile) {
//   const element = allFile[key].default;
//   componentsAll[element.name] = element;
// }
export default {
  name: 'ExamineDialog',
  components: {
    eleUpload,
    TranspondLog,
    ExpensesClaimForm,
    ExamineOpinion,
    FlowLog,
    Postscript,
    workTarget,
    ManagePerformence,
    MonthlyPerformence,
    monthPerfSum,
    reserveMonthSum,
    CompanyAmountAdjustForm,
    DetailBudgetForm,
    MonthBudgetForm,
    ProjectBudgetForm,
    GroupFinance,
    companyMonthlyFinance,
    CheckFlowNodeDetail,
    companyMonthlyFinance,
    NewEvent,
    FormCommonFlowPost,
    StaffAnnualAssessmentNoForm
  },
  mixins: [mixin],
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    // 新增：add，编辑：edit，查看：check
    operaType: {
      type: String,
      default: ''
    },
    isExamine: {
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
    companyId: {
      type: String,
      default: ''
    },
    showFlowLog: {
      type: Boolean,
      default: true
    },
    flowId: {
      // 业务id
      type: String,
      default: ''
    },
    flowInstanceId: {
      type: String,
      default: ''
    },
    jobTaskId: {
      type: String,
      default: ''
    },
    flowNodeType: {
      type: String,
      default: ''
    },
    nextNodeName: {
      type: String,
      default: ''
    },
    nextNodeProxyId: {
      type: String,
      default: ''
    },
    searchFlowType: {
      type: String,
      default: ''
    },
    flowNodeProxyId: {
      type: String,
      default: ''
    },
    flowProxyId: {
      // 流程id
      type: String,
      default: ''
    },
    actionType: {
      type: String,
      default: ''
    },
    flowType: {
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
    formId: {
      type: String,
      default: ''
    },
    auditPassLogicFlag: {
      type: Boolean,
      default: true
    },
    tracking:{
      type: Boolean,
      default: false
    },
    hideTrackingButton: {
      type: Boolean,
      default: false
    },
    flowInstanceBizRelevanceList:{
      type: Array,
      default:function(){
        return [];
      }
    },
    isTimeoutPage:{ //是否是超时跳过的页面
      type:Boolean,
      default:false
    }
    // editId: { // 编辑的业务id
    //   type: String,
    //   default: ''
    // }
  },
  data() {
    return {
      detectZoom,
      attachFile: [],
      active: 0,
      stepData: [],
      jsonData: {},
      editData: {},
      checkboxPersonGroup: [],
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
            return createElement(
              'div',
              {
                style: {
                  whiteSpace: 'pre-wrap!important'
                }
              },
              scope.row.executeDesc
            );
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
      params: {},
      isShowFlowLog: true, // 是否显示流程日志，如果当前是追加的流程，只显示当前追加的流程日志，不显示初始数据的流程日志
      tableData: [],
      rightTabShow: true,
      checkViewFlowDetailVisible: false,
      checkFlowId: '',
      // nextNodeName: '',
      // flowNodeType: '',
      // nextNodeProxyId: '',
      // nodeChooseVisible: false,
      // checkboxPersonGroup: []
      transpondFlowData:[],
      isTranspondFlow: false, // 是否是转发流程
      formAndTranspondData:{},
      copyTemplateData:{},
      outTracking:'', //外部传入的流程跟踪状态
      allFlowNodeDetail:{},
      // isPrintPage:false
    };
  },
  computed: {
    currentComponent() {
      if (this.isDialog) {
        return 'el-dialog';
      } else {
        return 'div';
      }
    },
    copyTemplateDataComputed(){
      console.log('copyTemplateDataComputed',this.copyTemplateData)
      return Object.keys(this.copyTemplateData).length
    }
  },
  provide() {
    return {
      prevStepHandle: this.handleClose,
      sumbitFlow: () => { },
      submitFlowFinal: () => { },
      jusgeCustomChoose: () => { }
    };
  },
  watch: {
    tracking:{
      handler(val){
        this.outTracking = val
      },
      immediate: true
    },
    visible: {
      handler(val) {
        if (val) {
          this.fetchLogData();
        }
      },
      immediate: true
    }
  },
  async created() {
    this.isExamine && this.getFlowDetailFindFormPerson();
    const editData = await this.getFormDataById(this.flowInstanceId);
    console.log('editData111111111',editData)
    if (editData.dynamicParam) { // 判断是否是转发流程(有表单)
      this.isTranspondFlow = true;
      this.transpondFlowData = editData.dynamicParam;
      this.formAndTranspondData = editData;
      // // 转发后的流程，为了沿用之前的流程逻辑，有些数据需要获取转发之前的数据
      // this.selectFlowType = editData.transpondFlowInstance.selectFlowType;
      // this.businessId = editData.transpondFlowInstance.businessId;
    }

    console.log('this.searchFlowType2',this.searchFlowType)
    this.getAttachmentList();

    this.getNewEventData(editData); // 获取新建事项数据

  },
  mounted() {
    this.getFileList();
    this.calcRight();
    setTimeout(() => {
      this.calcRight();
    }, 1000);
    window.addEventListener('resize', this.calcRight);
  },
  destroyed() {
    window.removeEventListener('resize', this.calcRight);
  },
  methods: {
    getFlowDetailFindFormPerson() { // 获取流程节点详情
      console.log('getFlowDetailFindFormPerson-方法')
      console.log('this.$attrs.clickRow',this.$attrs.clickRow)
      // const url = Api.schedule.getFlowInstanceTemplateNode;
      const url = this.$attrs.clickRow.flowInstanceId ? Api.schedule.getFlowInstanceTemplateNode : Api.schedule.flowTemplateFindById;
      console.log('getFlowDetailFindFormPerson-url',url)
      this.$axios.post(url, { data: { id: this.$attrs.clickRow.flowProxyId }},
        res => {
          if (res.isSuccess) {
            this.allFlowNodeDetail = res.data;
          }
        }
      );
    },
    getNewEventData(editData){
      // 用于添加事项流程，需要获取模板（因为数据存在模板里面）
      // getTaskFormDetail: '/web/formProxy/findById', //  查询已建任务的表单模板
      // getFormDetail: '/web/formTemplateApi/findById', // 获取表单详情

      setTimeout(x=>{
        this.copyTemplateData = editData;
        console.log('this.copyTemplateData',this.copyTemplateData)
      },300)
      // this.$axios.post(
      //   Api.qualityManage.getTaskFormDetail,
      //   {
      //     data: {
      //       id: this.formId
      //     }
      //   },
      //   (res) => {
      //     if (res.isSuccess) {
      //       let templateData = res.data.templateData;
      //       this.copyTemplateData = JSON.parse(templateData);
      //       console.log('this.copyTemplateData',this.copyTemplateData)
      //     }
      //   }
      // );
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

    handleCheckFlow() {
      // 查看流程
      this.checkFlowId = this.flowProxyId;
      this.checkViewFlowDetailVisible = true;
    },
    // handleCheckFlow() {
    //   var pt = this.$parent;
    //   pt.handleCheckFlow(pt.clickRow);
    // },
    calcRight() {
      this.$nextTick(() => {
        const rightSideDom = document.querySelector('.examine-content .right-side');
        const expandBaBtnDom = document.querySelector('.examine-content .flex-right');
        if (rightSideDom && expandBaBtnDom) {
          const width = rightSideDom.clientWidth;
          expandBaBtnDom.style.right = (width - 20) + 'px';
        }
      });
    },
    flexRight() {
      // this.rightTabShow = this.rightTabShow ? false : true
      if (this.rightTabShow) {
        this.rightTabShow = false;
      } else {
        this.rightTabShow = true;
        this.calcRight();
      }
      if (this.searchFlowType == 'monthly_perf') {
        this.$nextTick(() => {
          // 修改月度绩效的尺寸
          this.$refs.monthly_perf.calculateKpiWidth();
          this.$refs.monthly_perf.calculateFoot();
        });
      }
    },
    printPage(list) {
      console.log('list,printPage',list)
      console.log('this.searchFlowType', this.searchFlowType)
      // if (this.searchFlowType == 'add_event_flow') {
      //   this.isPrintPage = true;
      //   setTimeout(x=>{
      //     this.isPrintPage = false;
      //   },1200)
      // }
      // return
      this.$refs.ExpensesClaimForm?.printPage(list);
      this.$refs[this.searchFlowType]?.printPage?.(list);
    },
    getFileList() {
      if (this.searchFlowType == 'DRAWINGS') {
        if (this.fileType == 'FILES') {
          this.$axios
            .post('/web/file/api/file/listByAll', {
              data: {},
              ids: this.fileIdS
            })
            .then((res) => {
              if (res.isSuccess) {
                this.tableData = res.data;
              }
            });
        } else {
          this.$axios
            .post('/web/project/api/drawing/getDrawingByDrawingIds', {
              data: { drawingIds: this.fileIdS }
            })
            .then((res) => {
              if (res.isSuccess) {
                this.tableData = res.data;
              }
            });
        }
      }
    },
    viewFile(row) {
      if (!row.absolutelyFileUrl) return;
      // window.open(
      //   `${viewFileUrl}/onlinePreview?url=${encodeURIComponent(
      //     this.$Base64.encode(row.absolutelyFileUrl)
      //   )}`
      // );
      viewFile(row.absolutelyFileUrl)
    },
    updateParams(params) {
      this.params = params;
    },
    beforeHandle() {
      // return this.$refs.detailBudgetForm;
      return this.$refs[this.searchFlowType];
      // console.log('this.$refs.detailBudgetForm', this.$refs.detailBudgetForm)
    },
    updateList(){
      this.$emit('success');
    },
    postExamine() {
      // this.bindBatchFileByIds();
      this.$emit('update:visible', false);
      this.$emit('success');
    },
    handleClose() {
      // this.approveForm.approveMessage = '';
      this.$emit('update:visible', false);
      // this.$emit('success');
      // this.models = {};
      // this.editData = {};
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
        (res) => {
          if (res.isSuccess) {
            this.personList = res.data.flowNodeAuditConfig.userVoList;
          }
        }
      );
    },
    // 选择节点人员
    handleCheckedPreson(val) { },
    // 取消选择节点人员
    handleCancelCheckNode() {
      this.nodeChooseVisible = false;
      this.checkboxPersonGroup = [];
    },
    // 获取审批节点人员
    handleGetNode() {
      this.nodeChooseVisible = false;
      this.handleBeforeSubmit('pass');
    },
    // 是否需要处理流程日志显隐
    showLog(val) {
      this.isShowFlowLog = val;
    }
  }
};
</script>

<style lang="scss" scoped>

.examine-div {
  display: inline-block;
}

.dialog-footer {
  position: fixed;
  bottom: 0;
  left: 0;
  width: 100%;
  padding: 25px 25px 30px;
  background: #fff;
  z-index: 1100;
}
::v-deep{
  .el-dialog__body{
    height: calc(100% - 30px);
    max-height:initial !important;
    min-height:initial !important;
    .examine-content{
      display: flex;
      height: 100%;
      justify-content: center;
      overflow: hidden;
      .left-side{
        display:flex;
        flex-direction:column;
        flex:1;
        min-width: 500px;
        padding: 10px 0;
        overflow: auto;
        .container{
          height: 100% !important;
        }
      }
      .right-side{
        min-width: 280px;
        padding: 10px 0;
        margin-left: 10px;
        display: flex;
        flex-direction: column;
        max-width:20%;
        .flow-wrap{
          width: 100%;
          padding: 0;
        }
      }
    }
  }
}
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
@media screen and (max-width: 1440px) {
    ::v-deep {
      .el-dialog__body{
        .examine-content{
          .right-side {
            min-width: 200px;
          }
        }
      }
  }
}
</style>
