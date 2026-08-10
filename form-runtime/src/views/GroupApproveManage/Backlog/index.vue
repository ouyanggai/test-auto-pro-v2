<!--
 * @Descripttion:待办
  * @Author: zhengzetao
 * @Date: 2022-06-15
-->
<template>
  <div class="container">
    <dy-table :fetchData="fetchData" :actions="actions" :keys="colKey" :list='tableData' :isPagination="true"
      :pagination="pagination" :height="height" showCheckBox ref="dytable" @selectDataEvent="selectDataEvent"></dy-table>
    <!-- <el-pagination v-if="pagination.total" background layout="total, sizes, prev, pager, next" style="text-align: right;"
      :page-size="pagination.size" @current-change="pageChange" @size-change="sizeChange" :total="pagination.total">
    </el-pagination> -->
    <!-- 审核弹窗(对固定页面的审核) -->
    <ExamineDialog v-if="ExpensesClaimFormVisible" ref="examineDialog" :visible.sync="ExpensesClaimFormVisible"
      :isExamine="isExamine" :operaType="operaType" :searchFlowType="searchFlowType" :flowId="flowId"
      :parallelNodeChooseList="parallelNodeChooseList" :manualChooseNodes="manualChooseNodes"
      :lastCountersignFlag="lastCountersignFlag" :flowInstanceId="flowInstanceId" :jobTaskId="jobTaskId"
      :flowNodeType="flowNodeType" :initiatorId="initiatorId" :nextNodeProxyId="nextNodeProxyId"
      :nextNodeName="nextNodeName" :flowProxyId="flowProxyId" :flowNodeProxyId="flowNodeProxyId" @success="fetchData"
      :actionType="actionType" :flowType="flowType" :auditPassLogicFlag="auditPassLogicFlag" :flowName="selectFlowName" :formId="formId"
      :isInitiator="isInitiator"  :clickRow="clickRow" :flowInstanceBizRelevanceList="flowInstanceBizRelevanceList"
      :tracking="tracking" :companyId="companyId"
      />

    <!-- 审核弹窗(对formMakiing制作的表单的审核) -->
    <EnterpriseExamineDialog v-if="examineDialogVisible" :visible.sync="examineDialogVisible" :btnVisible="btnVisible" :isExamine="isExamine" :actionType="actionType" :operaType="operaType"
      :businessId="businessId" :companyId="companyId" :flowId="flowId" :flowNodeType="flowNodeType" :nextNodeProxyId="nextNodeProxyId" :formId="formId"
      :parallelNodeChooseList="parallelNodeChooseList" :manualChooseNodes="manualChooseNodes" :clickRow="clickRow"
      :lastCountersignFlag="lastCountersignFlag" :nextNodeName="nextNodeName" :currentPendingNodeName="currentPendingNodeName" :flowNodeProxyId="flowNodeProxyId"
      :jobTaskId="jobTaskId" :initiatorId="initiatorId" :flowInstanceId="flowInstanceId" :selectFlowType="selectFlowType" :batchNo="batchNo"
      :auditPassLogicFlag="auditPassLogicFlag" @success="fetchData" :flowName="selectFlowName" :flowInstanceBizRelevanceList="flowInstanceBizRelevanceList"
      :isInitiator="isInitiator" :flowProxyId="flowProxyId" :tracking="tracking"
      />
    <!-- 移交-选择审批人 -->
    <PersonSelectDialog :visible.sync="handOverNodeVisible" v-if="handOverNodeVisible" @getSelectPerson="getSelectPerson">
    </PersonSelectDialog>

    <CheckFlowNodeDetail v-if="checkViewFlowDetailVisible" :dialogVisible.sync="checkViewFlowDetailVisible"
      :flowInstanceId="flowInstanceId" :flowId="checkFlowId" :initiatorId="initiatorId" :companyId="companyId"></CheckFlowNodeDetail>


    <!-- <FlowTranspondDialog v-if="transpondVisible" :dialogVisible.sync="transpondVisible"
      :flowInstanceId="flowInstanceId" :flowId="checkFlowId" :initiatorId="initiatorId"></FlowTranspondDialog> -->
    <!-- 流程转发 -->
    <el-dialog title="转发" :visible="transpondVisible" :append-to-body="true" :close-on-click-modal="false"
      @close='handleCloseTranspond' width="500px">
      <el-form
        :model="approveForm"
        :rules="rules"
        label-width="80px"
        ref="approveForm"
      >
        <el-form-item label="选择人员" prop="personName">
          <el-input
            v-model="approveForm.personName"
            placeholder="请选择人员"
            readonly
            @focus="openTranspondPerson"
          ></el-input>
        </el-form-item>
        <el-form-item label="附言" prop="approveMessage">
          <el-input
            type="textarea"
            :autosize="{ minRows: 2, maxRows: 7 }"
            maxlength="200"
            show-word-limit
            v-model="approveForm.approveMessage"
            placeholder="请填写附言"
          ></el-input>
        </el-form-item>
        <el-form-item label="" v-if="showSelectGroup">
          <el-checkbox-group v-model="approveForm.checkList">
            <el-checkbox label="fuyan">转发原附言</el-checkbox>
            <el-checkbox label="yijian">转发原意见</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
      </el-form>
      <div slot="footer" class="dialog-footer">
        <el-button @click="handleCloseTranspond">取 消</el-button>
        <el-button type="primary" @click="confirmTranspond">确 定</el-button>
      </div>
    </el-dialog>

    <!-- 选择转发人员 -->
    <!-- :selectUserCompanyId="selectUserCompanyId" :departmentId="departmentId" :isRelative="isRelative" -->
    <IndicatorHeaderDialog :visible.sync="indicatorHeaderVisible" :fielSelectType="fielSelectType" :companyId="companyId"
      v-if="indicatorHeaderVisible" @selectHeader="selectHeader"/>

    <!-- 批量审核弹框 -->
    <el-dialog title="批量处理" :visible="visible" :append-to-body="true" :close-on-click-modal="false"  center
      @close='handleClosebatch' width="500px" top="35vh">
      <el-row>
        <el-col :span="5" style="font-weight: 600;text-align: right;">审核结果：</el-col>
        <el-col :span="19">
          <el-radio-group v-model="auditStatus">
            <el-radio label="pass">同意</el-radio>
            <el-radio label="roll_back">回退上一节点</el-radio>
            <el-radio label="no_pass">不同意</el-radio>
          </el-radio-group>
        </el-col>
      </el-row>
      <el-row style="margin-top: 15px;">
        <el-col :span="5" style="font-weight: 600;text-align: right;">审核意见：</el-col>
        <el-col :span="19">
          <el-input
            type="textarea"
            placeholder="请输入内容"
            v-model="approveMessage"
          >
          </el-input>
        </el-col>
      </el-row>
      <div slot="footer" class="dialog-footer">
        <el-button @click="handleClosebatch">取 消</el-button>
        <el-button type="primary" @click="confirmBatch">确 定</el-button>
      </div>
    </el-dialog>
    <!-- 批量审核结果弹框 -->
    <el-dialog title="系统提示" v-if="resultVisible" :visible="resultVisible" :append-to-body="true" :close-on-click-modal="false"  center
      @close='handleCloseResult' width="500px" :show-close="false" top="35vh">
      <div style="max-height: 530px;overflow: auto;">
        <div>
          已处理 <span style="font-weight: 600;">{{ completeNum }}/{{ allNum }}</span> 个，
          成功处理 <span style="font-weight: 600;color: #409EFF;">{{ completeNum - failList.length <= 0 ? 0 : completeNum - failList.length }}</span> 个
          <span v-if="failList.length">，以下 <span style="font-weight: 600;color: #F56C6C;">{{ failList.length }}</span> 个不能进行批量处理</span>
        </div>
        <div>
          <div v-for="(val,index) in failList" :key="index">
            <span>《{{ val.flowName }}》</span><span class="error">原因：{{ val.message}}</span>
          </div>
        </div>
      </div>
      <div slot="footer" class="dialog-footer">
        <el-button @click="handleCloseResult" v-if="isComplete">关 闭</el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script>
import DyTable from '@/components/DyTable';
import ExamineDialog from '../components/ExamineDialog';
// import EnterpriseExamineDialog from '../components/EnterpriseExamineDialog';
import PersonSelectDialog from '../components/PersonSelectDialog.vue';
import CheckFlowNodeDetail from '../components/CheckFlowNodeDetail.vue';
import IndicatorHeaderDialog from '@/components/IndicatorHeaderDialog.vue'
import moment from 'moment';

import Api from '@/api';
import mixin from '@/views/GroupApproveManage/components/flowTypeMixin'
import { deepClone } from '@/utils';
import { localstorageGet } from '@/utils/auth';
import {
  Loading
} from 'element-ui';

const EnterpriseExamineDialog = () => import('@/views/GroupApproveManage/components/EnterpriseExamineDialog.vue');
export default {
  name: '',
  mixins:[mixin],
  components: { DyTable, ExamineDialog, EnterpriseExamineDialog, IndicatorHeaderDialog,PersonSelectDialog, CheckFlowNodeDetail },
  props: {
    flowName: {
      type: String,
      default: ''
    },
    startDate: {
      type: String,
      default: ''
    },
    endDate: {
      type: String,
      default: ''
    },
    initiator: {
      type: String,
      default: ''
    },
    status: {
      type: String,
      default: ''
    },
    sFlowTypeList: {
      type: Array,
      default: () => {
        return [];
      }
    },
    isAssociateFlow: {
      type: Boolean,
      default: false
    }
  },
  data() {
    const preAuditStatus = {
      'pass':'通过',
      'transfer':'移交',
      'no_pass':'通过',
      'roll_back_the_previous_level':'回退上一级',
      'abandon':'丢弃',
      'retrieve':'回退',
      'withdraw':'撤销'
    }
    return {
      companyId:'',
      searchFlowType: '',
      auditPassLogicFlag: '',
      tableData: [],
      clickRow: {},
      btnVisible: false,
      examineDialogVisible: false,
      handOverNodeVisible: false,
      nextNodeName: '',
      currentPendingNodeName: '',
      flowNodeType: '',
      nextNodeProxyId: '',
      jobTaskId: '',
      colKey: {
        flowInstanceName: {
          label: '标题',
          showTooltip: true,
          minWidth: '430',
          handle: (scope, createElement) => {
            this.clickRow = scope.row;
            const flowName = scope.row.flowInstanceName || scope.row.formName;
            let click = ()=>{this.editAction(scope.row, true)}
            return createElement('el-link',{props:{type:'primary',underline:false},on:{click}},flowName);
          }
        },
        // flowName: {
        //   label: '流程名称',
        //   showTooltip: true,
        //   minWidth: '150'
        // },
        // auditWay: '流程名称',
        initiator: {
          label: '发起人',
          minWidth: '80'
        },
        initiatorDate: {
          label: '发起时间',
          showTooltip: true,
          minWidth: '160'
        },
        preAuditStatus: {
          label: '上一状态',
          minWidth: '100',
          handle: (scope, createElement) => {
            return createElement('span', preAuditStatus[scope.row.preAuditStatus]);
          }
        },
        preExecuteName: {
          label: '上一处理人',
          minWidth: '100'
        },
        preExecuteDate: {
          label: '上一处理时间',
          showTooltip: true,
          minWidth: '160'
        },
        currentPendingNodeName: {
          label: '当前审批节点',
          showTooltip: true,
          minWidth: '110'
        }
      },
      actions: [
        // 暂时注释：下次发版打开
        {
          label: '审核',
          width: this.isAssociateFlow ? '80' : '280',
          actionFixed:'right',
          size:'medium',
          handle: (scope, createElement) => {
            // const name = scope.row.currentPendingNodeType == 'common' ? '审核':'协同';
            const name = this.isAssociateFlow ? '详情' : (scope.row.currentPendingNodeType == 'common' ? '审核':'协同');
            let click = ()=>{this.editAction(scope.row, true)}
            return createElement('el-button',{props:{type:'text'},on:{click}},name);
          }
        },
        // {
        //   label: '审核',
        //   width: '280',
        //   actionFixed:'right',
        //   size:'medium',
        //   action: row => {
        //     this.clickRow = row;
        //     console.log(this.clickRow,'this.clickRow')
        //     this.editAction(row, true);
        //   }
        // },
        {
          label: '回退上一节点',
          size:'medium',
          action: row => {
            this.clickRollBack(row);
          },
          noShowActionButton: this.isAssociateFlow // 从关联流程公共组件进来的不显示按钮
        },
        {
          label: '移交',
          size:'medium',
          action: row => {
            this.clickHandOver(row);
          },
          noShowActionButton: this.isAssociateFlow
        },
        {
          label: '查看流程',
          size:'medium',
          action: row => {
            this.handleCheckFlow(row);
          },
          noShowActionButton: this.isAssociateFlow
        }
      ],
      pagination: {
        total: 0,
        pages: 1,
        size: 10
      },
      ExpensesClaimFormVisible: false,
      checkViewFlowDetailVisible: false,
      operaType: '',
      flowId: '', // 绑定的业务id
      isExamine: false,
      lastCountersignFlag: false,
      searchFlowTypeName: {
        expense_budget: '费用报销',
        expense_reimbursement: '公司预算金额调剂',
        company_annual_budget: '公司年度预算',
        add_annual_budget: '追加公司年度预算',
        depart_monthly_budget: '部门月度预算',
        project_setup_budget: '项目立项预算',
        add_project_budget: '追加项目预算',
        enterprise: ''
      },
      flowProxyId: '',
      initiatorId: '',
      flowNodeProxyId: '',
      parallelNodeChooseList: [], // 审批时下一节点并行且存在自选人
      manualChooseNodes: [], // 审批时下一节点选择分支
      editId: '',
      actionType: '',
      flowType: '',
      height: null,
      query: {},
      visible:false,
      auditStatus:'',
      approveMessage:'',
      failList:[],
      resultVisible:false,
      completeNum:0,
      allNum:0,
      isComplete:false,
      flowInstanceBizRelevanceList:[],
      isInitiator:false,
      tracking: false
      // 移到mixin
      // formExist: '',
      // selectFlowName:'', //prop的flowName好像没值
      // selectFlowType: '',
      // formId: '',
      // flowInstanceId: '', // 流程实例id
      // businessId:'',
    };
  },
  computed: {
  },
  watch: {
    sFlowTypeList: {
      handler: function () {
        this.pagination = {
          total: 0,
          pages: 1,
          size: 10
        };
        this.fetchData();
      },
      deep: true
    }
  },
  created() { },
  async mounted() {
    const clientHeight = document.querySelector('.content-container').clientHeight;
    this.height = clientHeight - 170;

    this.$bus.$off('messageCenterSuccess');
    this.$bus.$on('messageCenterSuccess', () => {
      console.log('监听messageCenterSuccess')
      this.fetchData();
    });
  },
  // if (['company_annual_budget','edit_company_annual_budget','add_annual_budget','depart_monthly_budget','project_setup_budget','add_project_budget','expense_budget','annual_perf','monthly_perf','travel_expense', 'contract_review', 'contract_pay_request', 'request_funds', 'refund_bid', 'invoice_apply'].indexOf(this.selectFlowType) > -1) {
  //           return true
  //         } else {
  //           return false
  //         }
  methods: {
    normalizeTrackingValue(value, fallback) {
      const target = value === undefined || value === null ? fallback : value;
      if (target === undefined || target === null) {
        return false;
      }
      if (typeof target === 'boolean') {
        return target;
      }
      if (typeof target === 'number') {
        return target !== 0;
      }
      if (typeof target === 'string') {
        const normalized = target.trim().toLowerCase();
        if (['1', 'true', 'yes', 'y', 'on', '是', '跟踪', '已跟踪', '已设为跟踪'].includes(normalized)) {
          return true;
        }
        if (['0', 'false', 'no', 'n', 'off', '否', '', '不跟踪', '未跟踪', '取消跟踪'].includes(normalized)) {
          return false;
        }
        return false;
      }
      return false;
    },
    syncTrackingState(row) {
      const tracking = this.normalizeTrackingValue(row?.tracking, row?.trackingFlag);
      this.tracking = tracking;
      return {
        ...(row || {}),
        tracking,
        trackingFlag: tracking,
      };
    },
    handleCloseResult(){
      this.fetchData()
      this.auditStatus = ''
      this.approveMessage = ''
      this.failList = []
      this.isComplete = false
      this.resultVisible = false
    },
    handleClosebatch(){
      this.visible = false
    },
    confirmBatch(){
      if(!this.auditStatus)return this.$message.error('请选择审批结果')
      let confirmTitle,confirmMsg
      if(this.auditStatus == 'roll_back'){
        confirmTitle = '确定要继续吗？'
        confirmMsg = '您将回退流程至上一审批节点'
      }else if(this.auditStatus == 'no_pass'){
        confirmTitle = '提示'
        confirmMsg = '不同意后，流程“驳回给发起人”，是否确认不同意？'
      }else if(this.auditStatus == 'pass'){
        confirmTitle = '提示'
        confirmMsg = '确认同意？'
      }
      this.$confirm(confirmMsg, confirmTitle, {
          closeOnClickModal: false,
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        }).then(() => {
          this.handleClosebatch()
          this.completeNum = 0, //处理过得条数，不管成功还是失败的
          this.allNum = this.batchSelectDatas.length //总的需要处理的条数
          this.resultVisible = true
          if(this.auditStatus == 'roll_back')this.handleBatch()
          else if(this.auditStatus == 'no_pass' || this.auditStatus == 'pass')this.auditBatchFlow(this.auditStatus)
        }).catch(() => { });
    },
    auditBatchFlow(auditStatus){
      const failList = []
      var loading = Loading.service({
        lock: true,
        background: 'rgba(255,255,255,0.1)'
      });
      var fun = async ()=>{
        let {flowInstanceName,flowInstanceId,jobTaskId} = this.batchSelectDatas[this.completeNum]
        let formDataMongoVo = null
        let formData = await this.getFormDataById(flowInstanceId)
        if(formData)formDataMongoVo = formData
        let res = await this.handleBatchFlow(auditStatus,flowInstanceName,jobTaskId,formDataMongoVo)
        if(!res.result)failList.push({flowName:res.flowName,message:res.message})
        this.completeNum ++
        if(this.completeNum < this.allNum){
          fun()
        }else{
          loading.close()
          this.isComplete = true
          this.$message.success('批量处理完成')
          this.failList = failList
        }
      }
      fun()
    },
    handleBatchFlow(auditStatus,flowName,jobTaskId,formDataMongoVo){
      return new Promise((resolve,reject)=>{
        const param = {
          data: {
            name: flowName,
            jobTaskId,
            auditRecord: {
              auditStatus,
              executeDesc: this.approveMessage
            }
          },
          nextAuditorList: [],
          formDataMongoVo: {
            data: formDataMongoVo
          }
        };
        this.$axios.post(
          Api.qualityManage.submitTask,
          param,
          (res) => {
            if (res.isSuccess) {
              resolve({result:true,flowName})
            } else {
              resolve({result:false,message:res.message,flowName})
            }
          },
          '',
          {noErrMsg:false,noLoading:true}
        );
      })
    },
    handleBatch(){
      // let completeNum = 0,allNum = this.batchSelectDatas.length
      const failList = []
      var loading = Loading.service({
        lock: true,
        background: 'rgba(255,255,255,0.1)'
      });
      var fun = async ()=>{
        let {flowInstanceName,flowInstanceId,jobTaskId} = this.batchSelectDatas[this.completeNum]
        let res = await this.clickBatchRollBack(flowInstanceName,flowInstanceId,jobTaskId)
        if(!res.result)failList.push({flowName:res.flowName,message:res.message})
        this.completeNum ++
        if(this.completeNum < this.allNum){
          fun()
        }else{
          loading.close()
          this.isComplete = true
          this.$message.success('批量处理完成')
          this.failList = failList
        }
      }
      fun()
    },
    clickBatchRollBack (flowName,flowInstanceId,jobTaskId) {
      return new Promise((resolve,reject)=>{
        this.$axios.post(
          Api.schedule.rollBackNode,
          {
            data: {
              id: flowInstanceId,
              jobTaskId,
              withdrawDesc:this.approveMessage
            }
          },
          (res) => {
            if (res.isSuccess) {
              resolve({result:true,flowName})
              // this.$message.success('已回退到上一审批节点');
              // this.nodeChooseVisible = false;
              // this.$emit('postExamine');
            } else {
              resolve({result:false,message:res.message,flowName})
            }
          },
          '',
          {noErrMsg:false,noLoading:true}
        ).catch(err=>{
          // console.log('err',err)
          // resolve({result:false,message:err.message,flowName})
        })
      })
    },
    batchAddApprove(){
      this.batchSelectDatas = this.$refs.dytable.selectDatas
      if(!this.batchSelectDatas.length){
        return this.$message.error('请选择流程')
      }
      this.visible = true
    },
    queryData(query) {
      this.query = {
        ...query
      };
      this.pagination.pages = 1;
      this.fetchData();
    },
    async isForm(formProxyId) {
      //判断无表单和有表单 如果有模板，就是有表单，没有模板信息就是固定页面无表单
      let res = await this.getFormData(formProxyId)
      if(res.templateData){
        return true
      }else{
        return false
      }
    },
    fetchData() {
      console.log('fetchData-待办')
      const data = {
        typeId: '',
        taskStatus: 'pending',
        auditWayList: [],//this.sFlowTypeList,
        useScope: 'invest',
        flowInstanceBizRelevance: {
          planId: null,
          stationId: null,
          resourcesId: null
        },
        flowInstanceBizRelevanceList: [
          {
            otherBiz: 'company',
            otherBizId: ''
          }
        ],
        ...this.query
      };
      this.$axios.post(
        Api.qualityManage.findList,
        {
          data,
          pagination: true,
          pages: this.pagination.pages,
          size: this.pagination.size
        },
        res => {
          if (res.isSuccess) {
            res.data.forEach(x => {
              x.name = this.searchFlowTypeName[x.auditWay] || x.formName;// (this.searchFlowType == 'expense_budget' ? '费用报销' : '公司预算金额调剂');
            });
            this.tableData = res.data ? res.data : [];
            this.pagination.total = res.total;
            
            // 检查当前页是否超出最大页数，如果超出则回到前一页
            const maxPage = Math.ceil(this.pagination.total / this.pagination.size);
            if (maxPage > 0 && this.pagination.pages > maxPage) {
              this.pagination.pages = maxPage; // 设置为最后一页
              // 重新获取最后一页数据
              this.$axios.post(
                Api.qualityManage.findList,
                {
                  data,
                  pagination: true,
                  pages: this.pagination.pages,
                  size: this.pagination.size
                },
                res2 => {
                  if (res2.isSuccess) {
                    res2.data.forEach(x => {
                      x.name = this.searchFlowTypeName[x.auditWay] || x.formName;
                    });
                    this.tableData = res2.data ? res2.data : [];
                  } else {
                    this.$message.error(res2.message);
                  }
                }
              );
            }
            // 加签bug调试暂时注释
            // 如果有打开的弹窗，检查是否需要更新弹窗数据
            // if (this.ExaminesClaimFormVisible || this.examineDialogVisible) {
            //   this.updateCurrentRowData();
            // }

            // this.originData = deepClone(res.data)
            // this.wholeData = res.data
            // this.tableData = this.generateTableData(this.wholeData)
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    async editAction(row, type) {
      row = this.syncTrackingState(row);
      this.clickRow = row;
      console.log('editAction',row)
      // if (row.typeName == '新建事项流程类型') { // 新建事项流程

      // } else {
        if (row.flowInstanceName.indexOf('原发') > -1) { // 转发流程
          const formExistType = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'transpond_formExist');
          this.formExist = formExistType?.otherBizId || '';
          const auditWayType = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'transpond_auditWay');
          row.auditWay = auditWayType?.otherBizId || '';
        } else { // 不是转发流程
          this.formExist = row.formExist;
        }
        console.log('this.formExist',this.formExist)
        this.parallelNodeChooseList = [];
        this.manualChooseNodes = []
        // this.formExist = row.formExist;
        this.auditPassLogicFlag = row.auditPassLogicFlag // 判断并行节点是否最后一人审批
        //判断是否是发起人，方便他写附言
        let createrId = row.initiatorId
        this.isInitiator = false
        console.log(createrId , localstorageGet('userId'))
        if(createrId == localstorageGet('userId')){
          this.isInitiator = true
        }
        if (!this.formExist) { // formMaking表单
          this.operaType = 'edit'; // 费用预算用的操作
          this.actionType = 'examine';
          this.isExamine = this.isAssociateFlow ? false : true; // 加多一个是否从关联流程组件进来的判断
          // this.isExamine = true;
          // this.lastCountersignFlag = row.lastCountersignFlag;// 判断是否为当前节点最后一个审批人--选择下一个分支节点
          this.initiatorId = row.initiatorId;
          this.btnVisible = type;
          this.flowId = row.flowProxyId;
          this.flowProxyId = row.flowProxyId;
          this.flowInstanceId = row.flowInstanceId;
          this.batchNo = row.batchNo;
          console.log('this.batchnorow',row.batchNo)
          console.log('this.batchno123',this.batchNo)
          this.formId = row.formProxyId;
          this.flowNodeProxyId = row.flowNodeProxyId;
          this.flowNodeType = row.flowNextNodeAuditType;
          this.nextNodeProxyId = row.nextNodeProxyId;
          this.nextNodeName = row.nextNodeName;
          this.currentPendingNodeName = row.currentPendingNodeName; // 当前审批节点
          this.jobTaskId = row.jobTaskId;
          this.selectFlowType = row.auditWay;
          this.selectFlowName = row.flowName || row.formName; // 不要用flowInstanceName，可能会出现叠加

          this.flowInstanceBizRelevanceList = deepClone(row.flowInstanceBizRelevanceList);
          // const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
          // this.businessId = find?.otherBizId || '';
          if (row.flowInstanceName.indexOf('原发') > -1) {
            const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == (row.auditWay+'_transpondFlow'));
            this.businessId = find?.otherBizId || '';
          } else {
            const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
            this.businessId = find?.otherBizId || '';
          }
          const company = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'company');
          this.companyId = company?.otherBizId || '';
          this.examineDialogVisible = true;
        } else { // 固定页面
          this.operaType = 'edit'; // 费用预算用的操作
          this.actionType = 'examine'; // 其他无表单流程的操作
          this.flowInstanceBizRelevanceList = deepClone(row.flowInstanceBizRelevanceList);
          if (row.auditWay == 'annual_perf'||row.auditWay == 'year_kpi_work_target') {
            const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'manageTarget');
            if (find) { // 管理指标
              this.flowType = 'manageTarget';
            } else { // 工作指标
              this.flowType = 'workTarget';
            }
          }
          this.searchFlowType = row.auditWay;
          // this.isExamine = row.flowInstanceName.indexOf('原发') > -1 ? false : true; // 如果有原发两个字，就是转发的流程，转发流程表单只能查看
          // this.isExamine = true;
          this.isExamine = this.isAssociateFlow ? false : true; // 加多一个是否从关联流程组件进来的判断
          this.lastCountersignFlag = row.lastCountersignFlag;// 判断是否为当前节点最后一个审批人--选择下一个分支节点
          this.initiatorId = row.initiatorId;
          if (row.flowInstanceBizRelevanceList.length == 1) {
            this.flowId = row.flowInstanceBizRelevanceList[0].otherBizId; // 业务id，绑定的什么业务就是什么
          } else {
            if (row.flowInstanceName.indexOf('原发') > -1) {
              const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == (row.auditWay+'_transpondFlow'));
              this.flowId = find?.otherBizId || '';
            } else {
              const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
              this.flowId = find.otherBizId;
            }
            // const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
            // this.flowId = find.otherBizId;
          }
          this.formId = row.formProxyId;
          this.flowNodeProxyId = row.flowNodeProxyId;
          this.flowInstanceId = row.flowInstanceBizRelevanceList[0].flowInstanceId;
          this.batchNo = row.batchNo;
          console.log('this.batchno无表单',this.batchNo)
          this.flowNodeType = row.flowNextNodeAuditType;
          this.nextNodeProxyId = row.nextNodeProxyId;
          this.nextNodeName = row.nextNodeName;
          this.jobTaskId = row.jobTaskId;
          this.flowProxyId = row.flowProxyId;
          this.selectFlowName = row.flowName || row.formName;

          // 获取companyId
          const companyFixed = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'company');
          this.companyId = companyFixed?.otherBizId || '';

          this.ExpensesClaimFormVisible = true;
          // if (row.typeName == '新建事项流程类型') {
          //   console.log('打开--新建事项流程类型')
          // } else {
          //   this.ExpensesClaimFormVisible = true;
          // }
        }
      // }
    },
    // 加签bug调试暂时注释
    // 更新当前行数据的方法
    // async updateCurrentRowData() {
    //   // 重新获取当前行数据，用于加签后更新flowNodeProxyId等信息
    //   if (!this.clickRow || !this.clickRow.flowInstanceId) return;

    //   const data = {
    //     typeId: '',
    //     taskStatus: 'pending',
    //     auditWayList: [],
    //     useScope: 'invest',
    //     flowInstanceBizRelevance: {
    //       planId: null,
    //       stationId: null,
    //       resourcesId: null
    //     },
    //     flowInstanceBizRelevanceList: [
    //       {
    //         otherBiz: 'company',
    //         otherBizId: ''
    //       }
    //     ]
    //   };

    //   try {
    //     const res = await new Promise((resolve, reject) => {
    //       this.$axios.post(
    //         Api.qualityManage.findList,
    //         {
    //           data,
    //           pagination: true,
    //           pages: 1,
    //           size: 100 // 获取足够多的数据以确保能找到当前行
    //         },
    //         response => {
    //           if (response.isSuccess) {
    //             resolve(response);
    //           } else {
    //             reject(response);
    //           }
    //         }
    //       );
    //     });

    //     if (res.isSuccess && res.data) {
    //       console.log('res.data',res.data)
    //       console.log('this.clickRow.flowInstanceId',this.clickRow.flowInstanceId)
    //       // 查找当前行的最新数据
    //       const updatedRow = res.data.find(item => item.flowInstanceId === this.clickRow.flowInstanceId);
    //       console.log('updatedRow',updatedRow)
    //       if (updatedRow) {
    //         // 更新clickRow数据
    //         this.clickRow = updatedRow;

    //         // 如果弹窗是打开的，通知弹窗组件更新数据
    //         if (this.examineDialogVisible || this.ExpensesClaimFormVisible) {
    //           this.$bus.$emit('updateExamineDialogData', updatedRow);
    //         }
    //       }
    //     }
    //   } catch (error) {
    //     console.error('更新当前行数据失败:', error);
    //   }
    // },
    resetParallelNodeChooseList(val) {
      this.parallelNodeChooseList = val;
    },
    // 点击-移交
    clickHandOver(row) {
      this.handOverNodeVisible = true;
      this.clickRow = row;
    },
    async handleCheckFlow(row) {
      // 查看流程
      this.selectFlowType = row.auditWay;
      this.checkFlowId = row.flowProxyId;
      // if (await this.isForm(row.formProxyId)) {
      this.flowInstanceId = row.flowInstanceId;
      // } else {
      //   this.flowInstanceId = row.flowInstanceBizRelevanceList[0].flowInstanceId;
      // }
      this.initiatorId = row.initiatorId
      const company = row.flowInstanceBizRelevanceList?.find(item => item.otherBiz == 'company');
      this.companyId = company?.otherBizId || '';
      this.checkViewFlowDetailVisible = true;
    },
    // 点击-回退上一节点
    // clickRollBack(row){
    //   this.$prompt('如确认回退上一节点，请填写备注', '提示', {
    //       confirmButtonText: '确定',
    //       cancelButtonText: '取消',
    //       inputPattern:/\S/,
    //       inputErrorMessage:'备注不能为空'
    //     }).then(({ value }) => {
    //       if(!value){
    //       }else{
    //         this.$message({
    //           type: 'success',
    //           message: '你的邮箱是: ' + value
    //         });
    //       }
    //     }).catch(() => {
    //       console.log('ddd')
    //     });
    // },
    // rollBack(row){
    //   return this.$axios.post(
    //     Api.schedule.rollBackNode,
    //     {
    //       data: {
    //         id: row.flowInstanceId,
    //         jobTaskId: row.jobTaskId,
    //         auditRecord: {
    //           auditStatus: '',
    //           executeDesc: '回退了'
    //         }
    //       }
    //     })
    // },
    clickRollBack(row) {
      this.$confirm('确定要继续吗？', '您将回退流程至上一审批节点', {
        closeOnClickModal: false,
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        this.$axios.post(
          Api.schedule.rollBackNode,
          {
            data: {
              id: row.flowInstanceId,
              jobTaskId: row.jobTaskId
              // auditRecord: {
              //   auditStatus: 'roll_back_the_previous_level',
              //   executeDesc: '回退了11111'
              // }
            }
          },
          (res) => {
            if (res.isSuccess) {
              this.$message.success('已回退到上一审批节点');
              this.fetchData();
            } else { }
          }
        );
      }).catch(() => { });
    },
    getSelectPerson(data) {
      if (data?.checkboxPersonGroup?.length) {
        this.handleHandOver(data.checkboxPersonGroup);
      } else {
        this.$message.warning('至少添加一位审批人');
      }
    },
    handleHandOver(userlist) {
      const nameList = userlist.map(item => item.name);
      const idList = userlist.map(item => item.id);
      const msg = '确定要将：' + nameList.join(',') + ' 添加为当前流程节点的审批人吗？';
      this.$confirm(msg, '移交', {
        closeOnClickModal: false,
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        this.$axios.post(
          Api.schedule.handOver,
          {
            data: {
              id: this.clickRow.flowInstanceId || '',
              jobTaskId: this.clickRow.jobTaskId || '',
              batchNo: this.clickRow.batchNo || '',
              auditRecord: {
                auditStatus: 'transfer',
                executeDesc: '移交'
              }
            },
            approverAppendVo: {
              flowNodeProxyId: this.clickRow.flowNodeProxyId || '',
              userIds: idList || []
            }
          },
          res => {
            if (res.isSuccess) {
              this.$message.success('添加成功');
              this.clickRow = {};
              this.fetchData();
            }
          }
        );
      }).catch(() => { });
    }
  }
};
</script>

<style scoped lang="scss">
.error{
  color: #F56C6C;
  margin-left: 10px;
}
</style>
