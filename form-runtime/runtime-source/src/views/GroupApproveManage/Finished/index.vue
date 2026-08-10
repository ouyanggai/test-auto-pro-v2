<!--
 * @Descripttion:已办
 * @Author: Calvin
 * @Date: 2021-11-02 10:27:19
-->
<template>
  <div class="container">
    <dy-table :fetchData="fetchData" :actions="actions" :keys="colKey" :list='tableData' :isPagination="true"
      :pagination="pagination" :height="height" showCheckBox ref="dytable" @selectDataEvent="selectDataEvent"></dy-table>
    <!-- <el-pagination v-if="pagination.total" background layout="total, sizes, prev, pager, next" style="text-align: right;"
      :page-size="pagination.size" @current-change="pageChange" @size-change="sizeChange" :total="pagination.total">
    </el-pagination> -->
    <!-- 查看弹窗(对固定页面的查看) -->
    <!-- <ExamineDialog :visible.sync="ExpensesClaimFormVisible" :isExamine="isExamine" :operaType="operaType"
      :searchFlowType="searchFlowType" :flowId="flowId" :flowInstanceId="flowInstanceId" :actionType="actionType" :flowType="flowType"
      v-if="ExpensesClaimFormVisible" :flowProxyId="flowProxyId"/> -->
      <ExamineDialog v-if="ExpensesClaimFormVisible" :visible.sync="ExpensesClaimFormVisible" :isExamine="isExamine"
      :operaType="operaType" :flowId="flowId" :flowInstanceId="flowInstanceId" :showFlowLog="showFlowLog"
      :searchFlowType="searchFlowType" :isInitiator="isInitiator" :actionType="actionType" :flowType="flowType"
      :flowProxyId="flowProxyId" :formId="formId" :tracking="tracking" :hideTrackingButton="hideTrackingButton" @success="fetchData" :initiatorId="initiatorId" :flowInstanceBizRelevanceList="flowInstanceBizRelevanceList"/>

    <!-- 查看弹窗(对formMakiing制作的表单的查看) -->
    <EnterpriseExamineDialog v-if="examineDialogVisible" :btnVisible="btnVisible" :isExamine="isExamine" :isReInitiate="isReInitiate" :flowId="flowId" :formId="formId"
      :flowNodeProxyId="flowNodeProxyId" :jobTaskId="jobTaskId" :flowInstanceId="flowInstanceId" :selectFlowType="selectFlowType" :batchNo="batchNo" :actionType="actionType" :operaType="operaType"
      :visible.sync="examineDialogVisible" :businessId="businessId" :companyId="companyId" :flowInstanceBizRelevanceList="flowInstanceBizRelevanceList" :initiatorId="initiatorId" :isInitiator="isInitiator" :tracking="tracking" :hideTrackingButton="hideTrackingButton" @success="fetchData"/>
    <!-- @success="fetchData"  -->

    <CheckFlowNodeDetail v-if="checkViewFlowDetailVisible" :dialogVisible.sync="checkViewFlowDetailVisible"
      :flowInstanceId="flowInstanceId" :flowId="checkFlowId" :initiatorId="initiatorId" :companyId="companyId"></CheckFlowNodeDetail>

    <!-- 查看费用报销弹窗 -->
    <!-- <div v-if="ExpensesClaimFormVisible">
      <el-dialog :visible.sync="ExpensesClaimFormVisible" title="费用明细" :modal-append-to-body="true"
        :close-on-click-modal="false" :custom-class="isExamine ? 'dialog-fullscreen isExamine' : 'dialog-fullscreen'"
        center @close="closed" :before-close="closed" width="800px">
        <ExpensesClaimForm :operaType="operaType" :id="flowId">
        </ExpensesClaimForm>
        <FlowLog :flowInstanceId="flowInstanceId"></FlowLog>
          <span slot="footer" class="dialog-footer">
            <ExamineOpinion v-if="isExamine"></ExamineOpinion>
          </span>
        </el-dialog>
      </div> -->
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
  </div>
</template>

<script>

import { approveManageFlowStatus } from '@/utils';
import ExamineDialog from '../components/ExamineDialog';
// import EnterpriseExamineDialog from '../components/EnterpriseExamineDialog';
import CheckFlowNodeDetail from '../components/CheckFlowNodeDetail.vue';
import IndicatorHeaderDialog from '@/components/IndicatorHeaderDialog.vue'

import DyTable from '@/components/DyTable';
import Api from '@/api';
import mixin from '@/views/GroupApproveManage/components/flowTypeMixin'
import {deepClone} from '@/utils'
import { localstorageGet } from '@/utils/auth';

const EnterpriseExamineDialog = () => import('@/views/GroupApproveManage/components/EnterpriseExamineDialog.vue');
export default {
  name: '',
  mixins:[mixin],
  components: { DyTable, ExamineDialog, EnterpriseExamineDialog, CheckFlowNodeDetail,IndicatorHeaderDialog },
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
      tableData: [],
      btnVisible: false,
      examineDialogVisible: false,
      checkViewFlowDetailVisible: false,
      flowNodeProxyId: '',
      jobTaskId: '',
      colKey: {
        name: {
          label: '标题',
          showTooltip: true,
          minWidth: '430',
          // handle: (scope,createElement) => {
          //   let flowName = scope.row.flowInstanceName || scope.row.formName
          //   return createElement('span', flowName);
          // }
          handle: (scope, createElement) => {
            const flowName = scope.row.flowInstanceName || scope.row.formName;
            let click = ()=>{
              this.clickRow = scope.row;
              this.previewHandle(scope.row, false);
            }
            return createElement('el-link',{props:{type:'primary',underline:false},on:{click}},flowName);
          }
        },
        tracking:{
          label:'跟踪状态',
          minWidth: '80',
          handle: (scope, createElement) => {
            let tracking,type
            if (scope.row.flowStatus === 'end') {
              tracking = '否'
              type = 'info'
            } else if(scope.row.tracking){
              tracking = '是'
              type = 'success'
            }else{
              tracking = '否'
              type = 'info'
            }
            // let tracking = scope.row.tracking ? '是' :'否'
            return createElement('el-tag',{props:{type}}, tracking);
          }
        },
        // formName:{
        //   label:'流程名称',
        //   minWidth: '150',
        // },
        // auditWay: '流程名称',
        initiator: {
          label: '发起人',
          minWidth: 80
        },
        initiatorDate: {
          label: '发起时间',
          showTooltip: true,
          minWidth:'160'
        },
        preExecuteName:{
          label:'上一处理人',
          minWidth:90,
        },
        // preExecuteDate: {
        //   label: '上一处理时间',
        //   showTooltip: true,
        //   minWidth:'160'
        // },
        executeDate: {
          label: '处理时间',
          showTooltip: true,
          minWidth:'160'
        },
        currentAuditUserInfo:{
          label:'当前处理人',
          minWidth: '100',
          handle: (scope, createElement) => {
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
        },
        flowStatus: {
          label: '当前状态',
          minWidth: 80,
          handle: (scope, createElement) => {
            return createElement('span', approveManageFlowStatus(scope.row.flowStatus));
          }
        }
        // executeDate: '当前处理时间'
      },
      actions: [
        {
          label: '查看',
          width: this.isAssociateFlow ? '120' : '220',
          actionFixed:'right',
          action: row => {
            this.clickRow = row;
            this.previewHandle(row, false);
          }
        },
        {
          label: '查看流程',
          action: row => {
            this.handleCheckFlow(row);
          },
          noShowActionButton: this.isAssociateFlow // 从关联流程公共组件进来的不显示按钮
        },
        {
          label: '取回',
          actionFixed:'right',
          handle: (scope, createElement, self) => {
            let row = scope.row
            this.clickRow = row;
            const click = () => {
              this.retrieveProcess(row, false);
            };
            if (scope.row.flowStatus == 'run') {
              return createElement('button', { class: 'el-button el-button--text el-button--small' }, [
                <span onClick={click}>
                  取回</span>
              ]);
            }
          },
          noShowActionButton: this.isAssociateFlow // 从关联流程公共组件进来的不显示按钮
        },
        {
          label: '设为跟踪',
          actionFixed:'right',
          handle: (scope, createElement, self) => {
            let row = scope.row
            this.clickRow = row;
            const click = () => {
              this.setTracking(row);
            };
            if(scope.row.flowStatus == 'run'){
              if (scope.row.tracking) {
                return createElement('button', { class: 'el-button el-button--text el-button--small' }, [
                  <span onClick={click}>
                    取消跟踪</span>
                ]);
            }else{
                return createElement('button', { class: 'el-button el-button--text el-button--small' }, [
                  <span onClick={click}>
                    设为跟踪</span>
                ]);
              }
            }
          }
        }
      ],
      pagination: {
        total: 0,
        pages: 1,
        size: 10
      },
      ExpensesClaimFormVisible: false,
      operaType: '',
      actionType:'',
      flowId: '', // 绑定的业务id
      isExamine: false,
      isReInitiate: false,
      // searchFlowTypeName: {
      //   expense_budget: '费用报销',
      //   expense_reimbursement: '公司预算金额调剂',
      //   company_annual_budget: '公司年度预算',
      //   add_annual_budget: '追加公司年度预算',
      //   depart_monthly_budget: '部门月度预算',
      //   project_setup_budget: '项目立项预算',
      //   add_project_budget: '追加项目预算',
      //   enterprise: ''
      // },
      flowType:'',
      height:null,
      initiatorId:'',
      checkFlowId:'',
      flowInstanceBizRelevanceList:[],
      tracking:'', //跟踪事项标识符
      hideTrackingButton:false,
      isInitiator:false,
      isSkip:false

      // 移到mixin
      // formExist: '',
      // selectFlowName:'', //prop的flowName好像没值
      // selectFlowType: '',
      // formId: '',
      // flowInstanceId: '', // 流程实例id
      // businessId:'',
    };
  },
  watch: {
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
  async created() {
    // let instanceData = await this.getFormInstance()
    // console.log('instanceData',instanceData)
    if (this.$route?.query?.fromMsg){
      this.skipOpDetail();
    }
  },
  async mounted() {
    let clientHeight = document.querySelector('.content-container').clientHeight
    this.height = clientHeight-170


  },
  methods: {
    shouldHideTrackingButton(row) {
      return row?.flowStatus === 'end' || row?.status === 'end' || row?.hideTrackingButton === true;
    },
    setTracking(row){
      let tracking = row.tracking
      let msg = tracking ? '取消跟踪此流程？' : '确认跟踪此流程？'
      let targetTrackingStatus = tracking ? false : true
      console.log('tracking',tracking)
      this.$confirm(msg,'提示').then(()=>{
        let data = {
          data:{
            id:row.flowInstanceId
          },
          tracking:targetTrackingStatus
        }
        this.$axios.post(Api.schedule.flowTracking,data,res=>{
          if(res.isSuccess){
            this.$message.success('操作成功')
            this.fetchData();
          }else{
            this.$message.error(res.message)
          }
        })
      }).catch(()=>{})

    },
    retrieveProcess(row){
      this.$confirm('确认取回','提示').then(()=>{
        let jobTaskId = row.jobTaskId;
        let flowInstanceId = row.flowInstanceId;
        let data = {
          jobTaskId,
          id:flowInstanceId
        }
        this.$axios.post(Api.approveManage.retrieveProcess,{data},res=>{
          if(res.isSuccess){
            this.$message.success('操作成功')
            this.fetchData();
          }else{
            this.$message.error(res.message)
          }
        })
        //
      }).catch(()=>{

      })
    },
    queryData(query) {
      this.query = {
        ...query
      }
      this.pagination.pages = 1
      this.fetchData()
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
      let data = {
        executorId: this.$store.state.user.userId,
        auditWayList: [],//this.sFlowTypeList,
        useScope: 'invest',
        typeId: '',
        taskStatus: 'done',
        flowInstanceBizRelevance: {
          planId: null,
          stationId: null,
          resourcesId: null
        },
        flowInstanceBizRelevanceList:[
          {
            otherBiz:'company',
            otherBizId:''
          }
        ],
        ...this.query
      }
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
            // res.data.forEach(x => {
            //   x.name = this.searchFlowTypeName[x.auditWay] || x.formName;// (this.searchFlowType == 'expense_budget' ? '费用报销' : '公司预算金额调剂');
            // });
            this.pagination.total = res.total;
            // this.originData = deepClone(res.data)
            // this.wholeData = res.data
            this.tableData = res.data//this.generateTableData(this.wholeData)
          } else {
            this.$message.error(res.message)
          }
        }
      );
    },
    async previewHandle(row,) {
      if (this.$route.query.id && this.isSkip) return;
      if (row.flowInstanceName.indexOf('原发') > -1) { // 转发流程
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

      // this.selectFlowType = row.auditWay
      // this.formExist = row.formExist
      // if (await this.isForm(row.formProxyId)) { // formMaking表单
        //判断是否是发起人，方便他写附言
      let createrId = row.initiatorId
      this.isInitiator = false
      if(createrId == localstorageGet('userId')){
        this.isInitiator = true
      }
      this.tracking = row.tracking
      this.hideTrackingButton = this.shouldHideTrackingButton(row)
      if (!this.formExist) { // formMaking表单
        this.operaType = 'check';
        this.actionType = 'preview'
        this.isExamine = false;
        this.isReInitiate = false;
        this.flowId = row.flowProxyId;
        this.formId = row.formProxyId;
        this.initiatorId = row.initiatorId;
        // this.initiatorId = row.initiatorId || row.createrId;
        this.flowNodeProxyId = row.flowNodeProxyId;
        this.flowInstanceId = row.flowInstanceId;
        this.batchNo = row.batchNo;
        this.jobTaskId = row.jobTaskId;
        this.selectFlowType = row.auditWay;
        this.examineDialogVisible = true;
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
      } else { // 固定页面
        this.operaType = 'check';
        this.actionType = 'preview' //非费用预算流程
        this.flowInstanceBizRelevanceList = deepClone(row.flowInstanceBizRelevanceList);
        if(row.auditWay == 'annual_perf'||row.auditWay == 'year_kpi_work_target'){
          let find = row.flowInstanceBizRelevanceList.find(item=>item.otherBiz == 'manageTarget')
          if(find){ //管理指标
            this.flowType = 'manageTarget'
          }else{ //工作指标
            this.flowType = 'workTarget'
          }
        }
        this.isExamine = false;
        if(row.flowInstanceBizRelevanceList.length == 1){
          this.flowId = row.flowInstanceBizRelevanceList[0].otherBizId; // 业务id，绑定的什么业务就是什么
        }else{
          if (row.flowInstanceName.indexOf('原发') > -1) {
            const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == (row.auditWay+'_transpondFlow'));
            this.flowId = find?.otherBizId || '';
          } else {
            const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
            this.flowId = find.otherBizId;
          }
          // let find = row.flowInstanceBizRelevanceList.find(item=>item.otherBiz == row.auditWay)
          // this.flowId = find.otherBizId
        }
        this.formId = row.formProxyId;
        this.initiatorId = row.initiatorId;
        this.flowInstanceId = row.flowInstanceBizRelevanceList[0].flowInstanceId;
        this.batchNo = row.batchNo;
        this.searchFlowType = row.auditWay;
        this.initiatorId = row.initiatorId;
        this.flowProxyId = row.flowProxyId;
        this.ExpensesClaimFormVisible = true;
      }
    },
    handleCheckFlow(row) {
      // 查看流程
      this.checkFlowId = row.flowProxyId;
      // if (!row.formExist) {
        this.flowInstanceId = row.flowInstanceId;
      // } else {
      //   this.flowInstanceId = row.flowInstanceBizRelevanceList[0].flowInstanceId;
      // }
      this.initiatorId = row.initiatorId
      this.checkViewFlowDetailVisible = true;
    },
    closed() {
      this.ExpensesClaimFormVisible = false;
    },
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
        this.skipPreviewHandle(row);
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
    async skipPreviewHandle(row, type) {
      console.log('skipPreviewHandle', row)
      if ((row.flowInstanceName && row.flowInstanceName.indexOf('原发') > -1) || (row.name && row.name.indexOf('原发') > -1)) { // 转发流程--已发流程名称没有flowInstanceName字段
        const formExistType = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'transpond_formExist');
        this.formExist = formExistType?.otherBizId || '';
        const auditWayType = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'transpond_auditWay');
        row.auditWay = auditWayType?.otherBizId || '';
        this.selectFlowType = row.auditWay
      } else { // 不是转发流程
        this.selectFlowType = row.auditWay
        this.formExist = row.formExist
      }

      if (!this.formExist) { // formMaking表单
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
        const company = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'company');
        this.companyId = company?.otherBizId || '';
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
        }
        this.formId = row.formProxyId;
        this.initiatorId = row.initiatorId || row.createrId;
        this.flowInstanceId = row.flowInstanceBizRelevanceList[0].flowInstanceId;
        this.batchNo = row.batchNo;
        this.initiatorId = row.initiatorId || row.createrId;
        this.searchFlowType = row.auditWay;
        this.flowProxyId = row.flowProxyId;
        this.ExpensesClaimFormVisible = true;
      }
    },
  }
};
</script>

<style scoped lang="scss"></style>
