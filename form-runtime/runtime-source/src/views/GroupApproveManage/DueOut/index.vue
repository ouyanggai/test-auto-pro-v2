<!--
 * @Author: junshao
 * @Date: 2023-03-24 11:12:11
-->
<template>
  <div class="container">
    <dy-table :fetchData="fetchData" :actions="actions" :keys="colKey" :list='tableData' :isPagination="true"
      :pagination="pagination" :height="height"></dy-table>
    <!-- <el-pagination v-if="pagination.total" background layout="total, sizes, prev, pager, next" style="text-align: right;"
      :page-size="pagination.size" @current-change="pageChange" @size-change="sizeChange" :total="pagination.total"
      :height="height">
    </el-pagination> -->
    <!-- 查看弹窗(对固定页面的查看) -->
    <ExamineDialog v-if="ExpensesClaimFormVisible" :visible.sync="ExpensesClaimFormVisible" :isExamine="isExamine"
      :isReInitiate="true" :operaType="operaType" :flowId="flowId" :flowInstanceId="flowInstanceId"
      :flowNodeType="flowNodeType" :showFlowLog="showFlowLog" :parallelNodeChooseList="parallelNodeChooseList"
      :manualChooseNodes="manualChooseNodes" :formId="formId" :searchFlowType="searchFlowType"
      :flowNodeProxyId="flowNodeProxyId" :noFormFlowInstanceId="noFormFlowInstanceId" :flowProxyId="flowProxyId"
      :initiatorId="initiatorId" @success="fetchData" :nextNodeName="nextNodeName" :actionType="actionType"
      :flowType="flowType" :flowName="flowName" :flowInstanceBizRelevanceList="flowInstanceBizRelevanceList"/>

    <!-- 查看弹窗(对formMakiing制作的表单的查看) -->
    <EnterpriseExamineDialog v-if="examineDialogVisible" :btnVisible="btnVisible" :isExamine="isExamine" :flowStatus="flowStatus"
      :isReInitiate="true" :flowId="flowId" :formId="formId" :flowNodeProxyId="flowNodeProxyId" :jobTaskId="jobTaskId"
      :flowInstanceId="flowInstanceId" :parallelNodeChooseList="parallelNodeChooseList" :actionType="actionType" :operaType="operaType"
      :manualChooseNodes="manualChooseNodes" :flowNodeType="flowNodeType" :visible.sync="examineDialogVisible"
      :initiatorId="initiatorId" @success="fetchData" :flowName="flowName" :nextNodeName="nextNodeName" :selectFlowType="selectFlowType"
      :businessId="businessId" :companyId="companyId" :flowInstanceBizRelevanceList="flowInstanceBizRelevanceList"/>

    <CheckFlowNodeDetail v-if="checkViewFlowDetailVisible" :dialogVisible.sync="checkViewFlowDetailVisible"
      :flowInstanceId="flowInstanceId" :flowId="checkFlowId" :isDraft="true" :initiatorId="initiatorId" :companyId="companyId"></CheckFlowNodeDetail>
  </div>
</template>

<script>
import Api from '@/api';
import DyTable from '@/components/DyTable';
import ExamineDialog from '../components/ExamineDialog';
import EnterpriseExamineDialog from '../components/EnterpriseExamineDialog';
import CheckFlowNodeDetail from '../components/CheckFlowNodeDetail.vue';
// import mixin from '@/views/ApproveManage/components/mixin'
import { deepClone } from '@/utils';
import { localstorageGet, localstorageRemove } from '@/utils/auth.js';
export default {
  name: '',
  // mixins: [mixin],
  components: { DyTable, ExamineDialog, EnterpriseExamineDialog, CheckFlowNodeDetail },
  data() {
    return {
      clickRow: {},
      selectFlowType:'',
      companyId:'',
      businessId:'',
      tableData: [],
      colKey: {
        name: {
          label: '标题',
          showTooltip: true,
          minWidth: '460',
          handle: (scope, createElement) => {
            this.clickRow = scope.row;
            const flowName = scope.row.flowInstanceName || scope.row.formName;
            let click = ()=>{this.clickReInitiate(scope.row)}
            return createElement('el-link',{props:{type:'primary',underline:false},on:{click}},flowName);
          }
        },
        // flowName: {
        //   label: '流程名称',
        //   minWidth: '150',
        //   showTooltip: true,
        // },
        statusName:{
          label:'流程状态',
          minWidth:'90',
        } ,
        initiator:{
          label:'发起人',
          minWidth:'80',
        },
        // initiatorDate: '提交时间'
        initiatorDate: {
          label: '提交时间',
          showTooltip: true,
          minWidth:'160',
        }
      },
      actions: [
        {
          label: '重新发起',
          width: '200px',
          actionFixed:'right',
          handle: (scope, createElement, self) => {
            const click = () => {
              this.clickRow = scope.row;
              this.clickReInitiate(scope.row);
            };
            let buttonName = '重新发起';
            if (scope.row.flowStatus == 'draft') buttonName = '编辑';
            return createElement('button', { class: 'el-button el-button--text el-button--mini' }, [
              <span onClick={click} >{buttonName}</span>
            ]);
          }
        },
        {
          label: '查看流程',
          action: row => {
            this.handleCheckFlow(row);
          }
        },
        {
          label: '删除',
          handle: (scope, createElement, self) => {
            if(scope.row.flowStatus == 'rejected'){
              return ''
            }else{
              const click = () => {
                this.clickDeleteFlow(scope.row);
              };
              return createElement('button', { class: 'el-button el-button--text el-button--mini' }, [
                <span onClick={click} style='color:#f56c6c;'>删除</span>
              ]);
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
      checkViewFlowDetailVisible: false,
      isExamine: false,
      operaType: '',
      initiatorId: '', // 发起人id
      flowId: '',
      flowInstanceId: '',
      flowNodeType: '',
      searchFlowType: '',
      showFlowLog: true,
      btnVisible: true,
      formId: '',
      flowNodeProxyId: '',
      jobTaskId: '',
      examineDialogVisible: false,
      parallelNodeChooseList: [],
      manualChooseNodes: [],
      noFormFlowInstanceId: '',
      actionType: '',
      flowType: '',
      flowProxyId: '',
      height: null,
      query: {},
      flowName: '',
      nextNodeName:'', //下一节点名称
      checkFlowId:'',
      flowStatus:'',
      flowInstanceBizRelevanceList:[]
    };
  },
  props: {
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
    }
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
  created() {
    if (localstorageGet('findObj')) {
      const findObj = localstorageGet('findObj');
      this.needFind = true;
      this.findObj = JSON.parse(findObj);
      localstorageRemove('findObj');
    }
  },
  mounted() {
    const clientHeight = document.querySelector('.content-container').clientHeight;
    this.height = clientHeight - 170;
  },
  methods: {
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
      if(!res)return true
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
        taskStatus: 'waiting_send',
        auditWayList: [],//this.sFlowTypeList,
        useScope: 'invest',
        flowInstanceBizRelevance: {},
        flowInstanceBizRelevanceList: [
          // {
          //   otherBiz: 'company',
          //   otherBizId: localstorageGet('companyId')
          // }
        ],
        ...this.query
      };
      // 获取待发数据
      this.$axios.post(
        Api.approveManage.getTaskList,
        {
          data,
          pagination: true,
          pages: this.pagination.pages,
          size: this.pagination.size
        },
        res => {
          if (res.isSuccess) {
            var flowStatusObj = {
              rejected: '驳回',
              withdraw: '撤销',
              draft: '草稿'
            };
            res.data.forEach(item => {
              item.statusName = flowStatusObj[item.flowStatus];
            });
            this.tableData = res.data;
            // this.tableData.map(item => {
            //   item.flowStatus = this.translateStatus(item);
            // });
            this.pagination.total = res.total;
            
            // 检查当前页是否超出最大页数，如果超出则回到前一页
            const maxPage = Math.ceil(this.pagination.total / this.pagination.size);
            if (maxPage > 0 && this.pagination.pages > maxPage) {
              this.pagination.pages = maxPage; // 设置为最后一页
              // 重新获取最后一页数据
              this.$axios.post(
                Api.approveManage.getTaskList,
                {
                  data,
                  pagination: true,
                  pages: this.pagination.pages,
                  size: this.pagination.size
                },
                res2 => {
                  if (res2.isSuccess) {
                    res2.data.forEach(item => {
                      item.statusName = flowStatusObj[item.flowStatus];
                    });
                    this.tableData = res2.data;
                  } else {
                    this.$message.error(res2.message);
                  }
                }
              );
            }
            // this.originData = deepClone(res.data)
            // let wholeData = this.wholeData = res.data
            // this.tableData = this.generateTableData(this.wholeData)
            // if (this.needFind) {
            //   let findArr = wholeData.filter(item => {
            //     return item.auditWay == this.findObj.type
            //   })
            //   if (findArr.length) {
            //     for (let i = 0; findArr[i]; i++) {
            //       let flowInstanceBizRelevanceList = findArr[i].flowInstanceBizRelevanceList
            //       let find = flowInstanceBizRelevanceList.find(item => item.otherBizId == this.findObj.bizId)
            //       if (find) {
            //         this.clickReInitiate(findArr[i])
            //         break
            //       }
            //     }
            //   }
            //   console.log('findArr', findArr)
            // }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 进入
    async clickReInitiate(row, type) {
      console.log('clickReInitiate',row)
      if (row.flowInstanceName && row.flowInstanceName.indexOf('原发') > -1) { // 转发流程
        const formExistType = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'transpond_formExist');
        this.formExist = formExistType?.otherBizId || '';
        const auditWayType = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'transpond_auditWay');
        row.auditWay = auditWayType?.otherBizId || '';
      } else { // 不是转发流程
        this.formExist = row.formExist;
      }

      this.flowStatus = row.flowStatus
      this.selectFlowType = row.auditWay;
      // 这里的接口和已提的接口不一样字段也不一样---参考审批功能
      this.parallelNodeChooseList = [];
      this.manualChooseNodes = [];
      if (row.nextNodeType == 'parallel') {
        // 下一节点为并行,取出其中需要自选的节点
        row.nextAuditNodeList.map(x => {
          if (x.flowNodeAuditConfig?.auditType == 'run_node_choose') {
            // 取出其中的审批人自选节点
            this.parallelNodeChooseList.push({
              nodeName: x.nodeName,
              id: x.id,
              auditType: x.flowNodeAuditConfig.auditType,
              nodeAuditList: []
            });
          } else if (x.flowNodeAuditConfig?.auditType == 'department_supervisor' || x.flowNodeAuditConfig?.auditType == 'branched_passage_manager') {
            // 取出其中的主管审批和副总审批的类型节点
            this.parallelNodeChooseList.push({
              nodeName: x.nodeName,
              id: x.id,
              auditType: x.flowNodeAuditConfig.auditType,
              nodeAuditList: []
            });
          }
        });
      }
      if (row.auditPassLogicFlag && row.branchExecuteType == 'custom_choose' && row.nextAuditNodeList.length > 1) {
        // 下一节点为手动分支
        row.nextAuditNodeList.map((x, index) => {
          this.manualChooseNodes.push({
            nextNodeTemplateId: x.id,
            nodeName: x.nodeName,
            nodeType: x.type, // 为处理空节点
            branchName: '分支' + (index + 1),
            auditType: x.flowNodeAuditConfig.auditType
          });
        });
      }
      this.flowName = row.flowName;
      // this.formExist = row.formExist;
      this.nextNodeName = row.nextNodeName;

      // if (await this.isForm(row.formProxyId)) {
      if (!this.formExist) { // formMaking表单
        // formMaking表单
        this.operaType = 'reEdit';
        this.actionType = 'edit';
        this.isExamine = false;
        this.isReInitiate = true;
        this.initiatorId = row.initiatorId;
        this.flowId = row.flowProxyId;
        this.formId = row.formProxyId;
        this.flowNodeProxyId = row.flowNodeProxyId;
        this.flowNodeType = row.flowNextNodeAuditType;
        this.flowInstanceId = row.flowInstanceId;
        this.jobTaskId = row.jobTaskId;
        this.examineDialogVisible = true;

        this.flowInstanceBizRelevanceList = deepClone(row.flowInstanceBizRelevanceList);
        const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
        this.businessId = find?.otherBizId || '';
        const company = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'company');
        this.companyId = company?.otherBizId || '';
      } else {
        // 固定页面
        this.operaType = 'reEdit';
        this.flowInstanceBizRelevanceList = deepClone(row.flowInstanceBizRelevanceList);
        if (row.auditWay == 'expense_budget') this.operaType = 'edit';
        this.actionType = 'edit';
        if (row.auditWay == 'annual_perf' || row.auditWay == 'year_kpi_work_target') {
          const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'manageTarget');
          if (find) { // 管理指标
            this.flowType = 'manageTarget';
          } else { // 工作指标
            this.flowType = 'workTarget';
          }
        }
        this.isExamine = false;
        this.initiatorId = row.initiatorId;
        this.flowNodeProxyId = row.flowNodeProxyId;
        this.flowNodeType = row.flowNextNodeAuditType || 'run_node_choose';
        // this.flowId = row.flowInstanceBizRelevanceList[0].otherBizId;
        if (row.flowInstanceBizRelevanceList.length == 1) {
          this.flowId = row.flowInstanceBizRelevanceList[0].otherBizId; // 业务id，绑定的什么业务就是什么
        } else {
          const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
          this.flowId = find.otherBizId;
        }
        this.flowProxyId = row.flowProxyId;
        this.formId = row.formProxyId;
        this.flowInstanceId = row.flowInstanceBizRelevanceList[0].flowInstanceId;
        this.noFormFlowInstanceId = row.flowInstanceId;
        this.searchFlowType = row.auditWay;
        this.ExpensesClaimFormVisible = true;
      }
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
      this.checkViewFlowDetailVisible = true;
    },
    clickDeleteFlow(row) {
      // console.log('row',row)
      // return
      // 点击删除待发流程
      this.$confirm('删除后数据不可恢复', '您确定要删除该条待发流程?', {
        closeOnClickModal: false,
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        this.$axios.post(
          Api.approveManage.taskFlowDelete,
          {
            data: {},
            ids: [row.flowInstanceId]
            // ids: this.flowInstanceIdList
          },
          res => {
            if (res.isSuccess) {
              this.$message.success('删除成功');

              if (row.flowInstanceBizRelevanceList.length) this.deleteBiz(row.flowInstanceBizRelevanceList, row.auditWay);
              this.fetchData();
            } else {
              this.$message.error(res.message);
            }
          }
        );
      }).catch(() => { });
    },
    deleteBiz(obj, auditWay) {
      //
      const typeName = auditWay;
      // if (typeName == 'company_monthly_budget' || typeName == 'annual_perf' || typeName == 'monthly_perf'
      //     || typeName == 'depart_monthly_budget' || typeName == 'company_annual_budget' || typeName == 'contract_compliance_review'
      //     || typeName == 'contract_seal_review' || typeName == 'monthly_perf_reserveTalent' || typeName == 'contract_payment_form'
      //     || typeName == 'contract_invoicing'
      //   ) {
        const index = obj.findIndex(item => item.otherBiz == typeName);
        if (index > -1) {
          obj[index].otherBizId;
          const id = obj[index].otherBizId;
          if(typeName == 'annual_perf' || typeName == 'monthly_perf'){
            // this.deletePerf(id);
          }else if(typeName == 'depart_monthly_budget' || typeName == 'company_annual_budget' || typeName == 'company_monthly_budget'){
            this.deleteBudget(id)
          } else if(typeName == 'contract_compliance_review'){ // 合规评审(删除流程后不删除业务数据，因为还有专门的草稿列表查看和重新发起)
            this.saveOrModifyContractInfo(id,"0",null)
          } else if(typeName == 'contract_seal_review'){ // 盖章评审(删除流程后不删除业务数据，因为还有专门的草稿列表查看和重新发起)
            this.saveOrModifyContractInfo(id,"1","1")
          } else if(typeName == 'monthly_perf_reserveTalent'){ // 后备英才轮岗考核表
            // this.deleteReserveTalent(id)
          } else if(typeName == 'contract_payment_form'){ // 合同付款申请单
            this.deleteContractPayment(id)
          } else if(typeName == 'contract_receipt_form'){ // 合同收款申请单
            this.deleteContractCollection(id)
          } else if(typeName == 'contract_invoicing'){ // 合同开票申请单
            this.deleteContractInvoicing(id)
          } else if(typeName == 'cost_funds_transactions'){ // 请款单（资金往来和投资款）
            this.deleteCostFundsTransactions(id)
          } else if(typeName == 'cost_funds_transfer'){ // 资金调拨单（资金上划和资金下拨）
            this.deleteCostFundsTransfer(id)
          }  else if(typeName == 'project_approval_budget'){ //删除项目立项
            this.$axios.post(
              Api.formMaking.projectApprovalBudgetDelete,
              { data: { id: id }});
          }else if(typeName == 'assets_buy_apply' || typeName == 'assets_transfer' ||typeName == 'assets_handle' ||typeName == 'assets_allocate'){
            this.$axios.post(
              Api.formMaking.fixedAssetsDelete,
              { data: { id: id }});
          }else if(typeName == 'expense_budget'){
            this.$axios.post(Api.budgetManage.expenseReimbursementDelete,
              { data: { id: id }});
          }
        }
      // }
    },
    // 业务-保存和修改合同信息
    saveOrModifyContractInfo(id,status,examineStatus){
      let url = Api.contractManage.contractInfo.modifyContract;
      let param = {
        id: id,
        "contractSubtableVo":{
          "stampStatus":null,  // 状态:0草稿1提交,传null代表还没走审核流程（合同盖章状态）
          "stampExamineStatus":null, // 审核状态:0审核中1审核成功2审核失败,传null代表还没走审核流程（合同盖章状态）
        },
        "status": status,  // 状态:0草稿1提交（合规审查状态）
        "examineStatus":examineStatus,  // 审核状态:0审核中1审核成功2审核失败（合规审查状态）
      }
      this.$axios.post(
        url,
        {
          data: param
        },
        res => {
          if (res.isSuccess) {
          } else {
          }
        }
      );
    },
    // 删除合同开票申请表单业务
    deleteContractInvoicing(id){
      this.$axios.post(
        Api.formMaking.contractInvoicingDelete,
        { data: { id: id }});
    },
    // 删除请款单（资金往来和投资款）业务
    deleteCostFundsTransactions(id){
      this.$axios.post(
        Api.formMaking.costFundsTransactionsDelete,
        { data: { id: id }});
    },
    // 删除资金调拨单（资金上划和资金下拨）业务
    deleteCostFundsTransfer(id){
      this.$axios.post(
        Api.formMaking.costFundsTransferDelete,
        { data: { id: id }});
    },
    // 删除合同付款表单业务
    deleteContractPayment(id){
      this.$axios.post(
        Api.formMaking.contractPaymentDelete,
        { data: { id: id }});
    },
    // 删除合同收款表单业务
    deleteContractCollection(id){
      this.$axios.post(
        Api.formMaking.contractCollectionDelete,
        { data: { id: id }});
    },
    // 删除轮岗考核表业务
    deleteReserveTalent(id){
      this.$axios.post(
        Api.formMaking.deleteReserveKpiGroup,
        { data: { id: id }});
    },
    // deleteContract(id){
    //   console.log('id-deleteContract',id)
    //   this.$axios.post(
    //     Api.contractManage.contractInfo.deleteContract,
    //     { data: { id: id }});
    // },
    deleteBudget(id){
      this.$axios.post(
        Api.annualBudget.budgetDelete,
        { data: { id: id }});
    },
    deletePerf(id) {
      this.$axios.post(
        Api.performance.deleteWorkTarget,
        { data: { id: id }});
    }
  }
};

</script>
<style lang='scss' scoped></style>
