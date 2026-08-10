<!--
 * @Descripttion:已提
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
    <!-- 查看弹窗(对固定页面的查看) -->
    <ExamineDialog v-if="ExpensesClaimFormVisible" :visible.sync="ExpensesClaimFormVisible" :isExamine="isExamine"
      :operaType="operaType" :flowId="flowId" :flowInstanceId="flowInstanceId" :showFlowLog="showFlowLog"
      :searchFlowType="searchFlowType" :isInitiator="true" :actionType="actionType" :flowType="flowType"
      :flowProxyId="flowProxyId" :formId="formId" :initiatorId="initiatorId" :flowInstanceBizRelevanceList="flowInstanceBizRelevanceList"
      :tracking="tracking"
      :companyId="companyId"
      :hideTrackingButton="hideTrackingButton"/>

    <!-- 查看弹窗(对formMakiing制作的表单的查看) -->
    <EnterpriseExamineDialog v-if="examineDialogVisible" :btnVisible="btnVisible" :isExamine="isExamine" :isReInitiate="isReInitiate" :flowId="flowId" :formId="formId"
      :flowNodeProxyId="flowNodeProxyId" :jobTaskId="jobTaskId" :flowInstanceId="flowInstanceId" :batchNo="batchNo" :actionType="actionType" :operaType="operaType"
      :visible.sync="examineDialogVisible" :isInitiator="true" :selectFlowType="selectFlowType" :businessId="businessId" :companyId="companyId"
      :flowInstanceBizRelevanceList="flowInstanceBizRelevanceList" :initiatorId="initiatorId" :tracking="tracking" :hideTrackingButton="hideTrackingButton"/>

    <CheckFlowNodeDetail v-if="checkViewFlowDetailVisible" :dialogVisible.sync="checkViewFlowDetailVisible"
      :flowInstanceId="flowInstanceId" :flowId="checkFlowId" :initiatorId="initiatorId" :companyId="companyId"></CheckFlowNodeDetail>

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

    <!-- 催办记录弹窗 -->
    <el-dialog title="催办记录" :visible.sync="urgeRecordVisible" width="90%" :append-to-body="true">
      <el-table :data="urgeRecordList" border>
        <el-table-column prop="reminderUserName" label="催办人" min-width="100"></el-table-column>
        <el-table-column prop="nodeName" label="催办节点" min-width="120"></el-table-column>
        <el-table-column prop="urgeUserName" label="被催办人" min-width="100"></el-table-column>
        <el-table-column prop="urgeDate" label="催办时间" min-width="160"></el-table-column>
        <!-- <el-table-column prop="messageStatus" label="消息状态" min-width="100">
          <template slot-scope="scope">
            <span>{{ scope.row.messageStatus === 'sent' ? '已发送' : '未发送' }}</span>
          </template>
        </el-table-column> -->
      </el-table>
      <el-pagination
        :total="urgeRecordPagination.total"
        :page-sizes="[10, 20, 50, 100]"
        :page-size="urgeRecordPagination.size"
        :current-page="urgeRecordPagination.pages"
        background
        layout="total, sizes, prev, pager, next"
        @size-change="handleUrgeRecordSizeChange"
        @current-change="handleUrgeRecordCurrentChange"
        style="margin-top: 20px;text-align: right"
      >
      </el-pagination>
      <div slot="footer" class="dialog-footer">
        <el-button @click="urgeRecordVisible = false">关 闭</el-button>
      </div>
    </el-dialog>

  </div>
</template>
<script>
import Api from '@/api';
import DyTable from '@/components/DyTable';
import FlowDialog from './components/FlowDialog';
import ExamineDialog from '../components/ExamineDialog';
// import EnterpriseExamineDialog from '../components/EnterpriseExamineDialog';
import CheckFlowNodeDetail from '../components/CheckFlowNodeDetail.vue';
import IndicatorHeaderDialog from '@/components/IndicatorHeaderDialog.vue'

import { approveManageFlowStatus, deepClone } from '@/utils';
import { localstorageGet } from '@/utils/auth';
import mixin from '@/views/GroupApproveManage/components/flowTypeMixin'

const EnterpriseExamineDialog = () => import('@/views/GroupApproveManage/components/EnterpriseExamineDialog.vue');
export default {
  name: '',
  mixins:[mixin],
  components: { DyTable, FlowDialog, ExamineDialog, EnterpriseExamineDialog, CheckFlowNodeDetail,IndicatorHeaderDialog },
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
    return {
      clickRow: {},
      companyId:'',
      searchFlowType: '',
      showFlowLog: true,
      tableData: [],
      // approveDialogVisible: false,
      colKey: {
        name: {
          label: '标题',
          showTooltip: true,
          minWidth: '480',
          // handle: (scope, createElement) => {
          //   const flowName = scope.row.name || scope.row.formName;
          //   return createElement('span', flowName);
          // },
          handle: (scope, createElement) => {
            const flowName = scope.row.name || scope.row.formName;
            let click = ()=>{
              this.clickRow = scope.row;
              this.previewHandle(scope.row, false);
            }
            return createElement('el-link',{props:{type:'primary',underline:false},on:{click}},flowName);
          }
        },
        // flowName: {
        //   label: '流程名称',
        //   showTooltip: true,
        //   minWidth: '150',
        // },
        // initiator: '发起人',
        status: {
          label: '状态',
          minWidth: '80',
          handle: (scope, createElement) => {
            return createElement('span', approveManageFlowStatus(scope.row.status));
          }
        },
        // createDate: '提交时间',
        createDate: {
          label: '提交时间',
          minWidth: '160',
          showTooltip: true
        },
        currentNodeName: {
          label: '当前节点',
          minWidth: '100',
          handle:(scope, createElement)=>{
            if(scope.row.status == 'end'){
              return createElement('span', '完结');
            }else{
              return createElement('span', scope.row.currentNodeName);
            }
          }
        },
        currentAuditUserInfo:{
          label:'当前处理人',
          minWidth: '100',
          handle: (scope, createElement) => {
            if(scope.row.status == 'end'){
              return createElement('span', '完结');
            }else{
              let currentAuditUserInfo = scope.row.currentAuditUserInfo || {}
              let strArr = []
              for(let key in currentAuditUserInfo){
                let userList = currentAuditUserInfo[key]?.userList || []
                userList.forEach(el=>{
                  strArr.push(el.name)
                })
              }
              let str = strArr.join(',')
              return createElement('el-tooltip',{props:{content:str}}, [<span>{str}</span>]);
            }
          }
        },
        urgeRecord: {
          label: '催办记录',
          minWidth: '100',
          handle: (scope, createElement) => {
            if (scope.row.urgeFlag) {
              const click = () => {
                this.showUrgeRecord(scope.row);
              };
              return createElement('el-button', {
                props: { type: 'text' },
                on: { click }
              }, '查看');
            } else {
              return createElement('span', '');
            }
          }
        },
      },
      actions: [
        {
          label: '详情',
          width: this.isAssociateFlow ? '80' : '200',
          actionFixed:'right',
          action: row => {
            this.clickRow = row;
            this.previewHandle(row, false);
          }
        },
        {
          label: '催办',
          handle: (scope, createElement, self) => {
            const click = () => {
              this.urgeFlow(scope.row);
            };
            if (scope.row.status === 'run') {
              return createElement('button', { class: 'el-button el-button--text el-button--small' }, [
                <span onClick={click}>催办</span>
              ]);
            }
          },
          noShowActionButton: this.isAssociateFlow
        },
        {
          label: '撤销',
          handle: (scope, createElement, self) => {
            const click = () => {
              this.repealAuditFlow(scope.row);
            };
            if (scope.row.status == 'run') {
              return createElement('button', { class: 'el-button el-button--text el-button--small' }, [
                <span onClick={click}>
                  撤销</span>
              ]);
            }
          },
          noShowActionButton: this.isAssociateFlow // 从关联流程公共组件进来的不显示按钮
        },
        {
          label: '查看流程',
          action: row => {
            this.handleCheckFlow(row);
          },
          noShowActionButton: this.isAssociateFlow // 从关联流程公共组件进来的不显示按钮
        }
      ],
      pagination: {
        total: 0,
        pages: 1,
        size: 10
      },
      ExpensesClaimFormVisible: false,
      operaType: '',
      actionType: '',
      flowId: '', // 绑定的业务id
      btnVisible: false,
      examineDialogVisible: false,
      flowNodeProxyId: '',
      jobTaskId: '',
      checkViewFlowDetailVisible: false,
      isExamine: false,
      isReInitiate: false,
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
      flowType: '',
      tracking: false,
      hideTrackingButton: false,
      height: null,
      initiatorId:'' , //发起人
      checkFlowId:'',
      flowInstanceBizRelevanceList:[],

      // 移到mixin
      // formExist: '',
      // selectFlowName:'', //prop的flowName好像没值
      // selectFlowType: '',
      // formId: '',
      // flowInstanceId: '', // 流程实例id
      // businessId:'',
      // 催办相关
      urgeRecordVisible: false,
      urgeRecordList: [],
      // 催办记录分页
      urgeRecordPagination: {
        total: 0,
        pages: 1,
        size: 10
      },
      currentUrgeRecordRow: null, // 当前查看催办记录的行

    };
  },
  watch: {
    // 创建流程前选择的流程类型
    // searchFlowType: function (val) {
    //   this.pagination = {
    //     total: 0,
    //     pages: 1,
    //     size: 10
    //   };
    //   this.fetchData();
    // }
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
  created() {
    if (this.$route?.query?.fromMsg){
      this.skipOpDetail();
    }
  },
  mounted() {
    const clientHeight = document.querySelector('.content-container').clientHeight;
    this.height = clientHeight - 170;
  },
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
    queryData(query) {
      this.query = {
        ...query
      };
      if(this.query.queryEndDate){
        this.query.endDate = this.query.queryEndDate
        delete this.query.queryEndDate
      }
      if(this.query.queryStartDate){
        this.query.startDate = this.query.queryStartDate
        delete this.query.queryStartDate
      }
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
    // 获取表单字段值
    getFormData(formProxyId) {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.qualityManage.getTaskFormDetail,
          {
            data: {
              id: formProxyId
            }
          },
          (res) => {
            if (res.isSuccess) {
              resolve(res.data);
            }
          }
        );
      });
    },
    fetchData() {
      const data = {
        useScope: 'invest',
        auditWayList: [],//this.sFlowTypeList,
        statusList:['await_sent','run','withdraw','termination','abandon','rejected','end'],
        flowInstanceBizRelevanceList: [
          {
            otherBiz: 'company',
            otherBizId:''
          }
        ],
        ...this.query
      };
      if(data.status)delete data.statusList
      else delete data.status
      this.$axios.post(
        Api.schedule.getFlowInstanceList,
        {
          data,
          pagination: true,
          pages: this.pagination.pages,
          size: this.pagination.size
        },
        res => {
          if (res.isSuccess) {
            // res.data.forEach(x => {
            //   x.name = this.searchFlowTypeName[x.auditWay] || x.formName;// (this.searchFlowType == 'expense_budget' ? '费用报销' : '公司预算金额调剂');
            // });
            this.pagination.total = res.total;
            // this.originData = deepClone(res.data)
            // this.wholeData = res.data
            this.tableData = res.data;// this.generateTableData(this.wholeData)
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // addApprove() {
    //   this.approveDialogVisible = true;
    // },
    // =============消息跳转表单处理================
    // 消息中心跳转查看表单详情
    async skipOpDetail(){
      console.log('skipOpDetail')
      if(this.$route?.query?.id){
        let instanceData = await this.getFormInstance(this.$route.query.id)
        console.log('this.$route.query.id',this.$route.query.id)
        console.log('instanceData',instanceData)
        // let row = arr.find(item=>item.flowInstanceId == this.$route.query.id);
        let row = instanceData[0]
        this.previewHandle(row);
        this.$router.replace({path: '/groupApproveManage',query: {}})
        this.isSkip = true;
      }
    },
    // 获取表单实例数据
    getFormInstance(instanceId) {
      console.log('getAllFormFlowDataByBizId')
      let data = {
        useScope: 'invest',
        auditWayList: [],//this.sFlowTypeList,
        initiator:"all",
        id:instanceId,
        // 3a59f0e73d9f411cbc5a29b213d71529
        // id:instanceId,
      };
      return new Promise((resolve,reject)=>{
        this.$axios.post(
          Api.schedule.getFlowInstanceList,
          {
            data,
            pagination: false,
          },
          res => {
            if (res.isSuccess) {
              resolve(res.data)
            } else {
            }
          }
        )
      })
    },
    // 查看
    async previewHandle(row, type) {
      console.log('previewHandle',row)
      row = this.syncTrackingState(row);
      this.hideTrackingButton = row.status === 'end';
      if ((row.flowInstanceName && row.flowInstanceName.indexOf('原发') > -1) || (row.name && row.name.indexOf('原发') > -1)) { // 转发流程--已发流程名称没有flowInstanceName字段
        const formExistType = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'transpond_formExist');
        this.formExist = formExistType?.otherBizId || '';
        // const originalFlowInstanceId = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'transpond_flowInstanceId');
        // this.flowInstanceId = originalFlowInstanceId?.otherBizId || '';
        const auditWayType = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'transpond_auditWay');
        row.auditWay = auditWayType?.otherBizId || '';
        this.selectFlowType = row.auditWay
      } else { // 不是转发流程
        this.selectFlowType = row.auditWay
        this.formExist = row.formExist
      }

      // this.selectFlowType = row.auditWay;
      // this.formExist = row.formExist;
      console.log('this.formExist', this.formExist);
      if (!this.formExist) { // formMaking表单
        this.operaType = 'check';
        this.actionType = 'preview';
        this.isExamine = false;
        this.isReInitiate = false;
        this.flowId = row.flowProxyId;
        this.formId = row.formProxyId;
        this.initiatorId = row.initiatorId || row.createrId;
        this.flowNodeProxyId = row.currentNodeProxyId;
        this.flowInstanceId = row.id;
        this.batchNo = row.batchNo;
        this.jobTaskId = row.jobTaskId;
        this.examineDialogVisible = true;

        this.flowInstanceBizRelevanceList = deepClone(row.flowInstanceBizRelevanceList);
        if ((row.flowInstanceName && row.flowInstanceName.indexOf('原发') > -1) || (row.name && row.name.indexOf('原发') > -1)) {
          const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == (row.auditWay+'_transpondFlow'));
          this.businessId = find?.otherBizId || '';
        } else {
          const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
          this.businessId = find?.otherBizId || '';
        }
        // const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
        // this.businessId = find?.otherBizId || '';
        const company = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'company');
        this.companyId = company?.otherBizId || '';
        console.log('this.companyId', this.companyId);
      } else { // 固定页面
        this.operaType = 'check';
        this.actionType = 'preview';
        if (row.auditWay == 'annual_perf'||row.auditWay == 'year_kpi_work_target') {
          const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'manageTarget');
          if (find) { // 管理指标
            this.flowType = 'manageTarget';
          } else { // 工作指标
            this.flowType = 'workTarget';
          }
        }
        this.flowInstanceBizRelevanceList = deepClone(row.flowInstanceBizRelevanceList);
        this.isExamine = false;
        if (row.flowInstanceBizRelevanceList.length == 1) {
          this.flowId = row.flowInstanceBizRelevanceList[0].otherBizId; // 业务id，绑定的什么业务就是什么
        } else {
          if ((row.flowInstanceName && row.flowInstanceName.indexOf('原发') > -1) || (row.name && row.name.indexOf('原发') > -1)) {
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
        this.initiatorId = row.initiatorId || row.createrId;
        this.flowInstanceId = row.flowInstanceBizRelevanceList[0].flowInstanceId;
        this.batchNo = row.batchNo;
        this.initiatorId = row.initiatorId || row.createrId;
        this.searchFlowType = row.auditWay;
        this.flowProxyId = row.flowProxyId;
        this.ExpensesClaimFormVisible = true;
        const company = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'company');
        this.companyId = company?.otherBizId || '';
        console.log('this.companyId', this.companyId);
      }
    },
    async handleCheckFlow(row) {
      // 查看流程
      this.selectFlowType = row.auditWay;
      this.checkFlowId = row.flowProxyId;
      // if (await this.isForm(row.formProxyId)) {
      this.flowInstanceId = row.id;
      // } else {
      //   this.flowInstanceId = row.flowInstanceBizRelevanceList[0].flowInstanceId;
      // }
      const company = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'company');
      this.companyId = company?.otherBizId || '';
      this.initiatorId = row.createrId
      this.checkViewFlowDetailVisible = true;

    },
    withDrawFlow(row) {
      this.$axios.post(
        Api.frameworkInfo.departmentFramework.flow.withDraw,
        {
          data: {
            id: row.id,
            withdrawDesc: ''
          }
        },
        res => {
          if (res.isSuccess) {
            this.$message.success(`撤销成功`);
            if (row.flowInstanceBizRelevanceList.length) this.changeStatus(row.flowInstanceBizRelevanceList, row.auditWay);
            this.fetchData();
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    filterWithdraw (data) {
      const len = data.length || 0;
      const arr = [];
      for (let i = len - 1; i >= 0; i--) {
        if (data[i].auditStatus == 'withdraw') break;
        arr.unshift(data[i]);
      }
      return arr;
    },
    // 撤销流程
    repealAuditFlow(row) {
      this.$confirm('是否撤销该审核流程?', '提示', {
        closeOnClickModal: false,
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        // if (row.auditWay == 'company_annual_budget') {
        //   this.fetchLogData(row.id).then(res => {
        //     const filterWithdrawData = this.filterWithdraw(res);
        //     if (filterWithdrawData.length > 1) {
        //       return this.$message.error('该流程已有节点完成审批，不可撤销');
        //     } else {
        //       this.withDrawFlow(row);
        //     }
        //   });
        // } else {
          this.withDrawFlow(row);
        // }
      }).catch(() => {

      });
    },
    // 撤销后修改业务状态
    changeStatus(list, auditWay) {
      const typeName = auditWay;
      if (typeName == 'annual_perf' || typeName == 'monthly_perf') {
        const index = list.findIndex(item => item.otherBiz == typeName);
        if (index > -1) {
          const id = list[index].otherBizId;
          this.$axios.post(
            Api.performance.updateKpiGroup,
            {
              data: { id, kpiGroupStatus: 'not_submitted' },
              sign: 'update'
            });
        }
      }
    },
    closed() {
      this.ExpensesClaimFormVisible = false;
    },
    // 添加催办方法
    async urgeFlow(row) {
      try {
        const res = await this.$axios.post('/web/urgeHandleRecord/sendUrgeMessage', {
          flowInstanceId: row.id,
          data: {}
        });
        if (res.isSuccess) {
          this.$message.success('催办成功');
          // 更新当前行的催办标志
          row.urgeFlag = true;
        } else {
          this.$message.error(res.message || '催办失败');
        }
      } catch (error) {
        this.$message.error('催办失败');
      }
    },
    // 显示催办记录
    async showUrgeRecord(row) {
      this.currentUrgeRecordRow = row;
      // 重置分页参数
      this.urgeRecordPagination = {
        total: 0,
        pages: 1,
        size: 10
      };
      try {
        await this.fetchUrgeRecordList();
        this.urgeRecordVisible = true;
      } catch (error) {
        this.$message.error('获取催办记录失败');
      }
    },
    
    // 获取催办记录列表（带分页）
    async fetchUrgeRecordList() {
      if (!this.currentUrgeRecordRow) return;
      
      try {
        const res = await this.$axios.post('/web/urgeHandleRecord/list', {
          data:{
            flowInstanceId: this.currentUrgeRecordRow.id
          },
          pagination: true,
          pages: this.urgeRecordPagination.pages,
          size: this.urgeRecordPagination.size
        });
        if (res.isSuccess) {
          this.urgeRecordList = res.data || [];
          this.urgeRecordPagination.total = res.total || 0;
        } else {
          this.$message.error(res.message || '获取催办记录失败');
        }
      } catch (error) {
        this.$message.error('获取催办记录失败');
      }
    },
    // 催办记录分页大小改变
    handleUrgeRecordSizeChange(val) {
      this.urgeRecordPagination.size = val;
      this.urgeRecordPagination.pages = 1;
      this.fetchUrgeRecordList();
    },
    // 催办记录当前页改变
    handleUrgeRecordCurrentChange(val) {
      this.urgeRecordPagination.pages = val;
      this.fetchUrgeRecordList();
    }
  }
};
</script>

<style scoped lang="scss"></style>
