<!--
 * @Descripttion: 费用明细列表
 * @Author: zhengzetao
 * @Date: 2022-08-24
-->

<template>
  <div>
    <el-dialog
      :visible.sync="detailVisible"
      title="费用明细"
      :modal-append-to-body="true"
      :close-on-click-modal="false"
      @close="closeDialog"
      :before-close="closeDialog"
      width="70%"
    >
      <div>
        <div class="top">
          <el-select
            v-model="query.departmentId"
            @change="selectDepartment"
            placeholder="请选择部门"
            clearable
            style="margin-right:10px"
            v-if="isShowDepartment"
          >
            <el-option
              v-for="item in departmentList"
              :key="item.id"
              :label="item.name"
              :value="item.id"
            >
            </el-option>
          </el-select>
          <el-select
            v-model="query.budgetId"
            @change="selectExpendType"
            v-if="isShowBudget"
            placeholder="请选择预算类型"
            clearable
            filterable
            style="margin-right:10px"
          >
            <el-option
              v-for="item in costTypeList"
              :key="item.id"
              :label="item.name"
              :value="item.id"
            >
            </el-option>
            <!-- <el-option v-if="!costTypeList.some(item=>item.id == query.budgetId )" :label="detailRow.departName" :value="query.budgetId"></el-option> -->
          </el-select>
          <el-date-picker
            v-model="query.date"
            @change="selectDate"
            type="datetimerange"
            range-separator="至"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            value-format="yyyy-MM-dd hh:mm:ss"
            style="margin-right:10px;width:330px;"
          >
          </el-date-picker>
        </div>
        <div
          class="content"
          style="padding:15px 0;"
        >
          <el-table
            :data="tableData"
            max-height="450"
          >
            <el-table-column
              type="index"
              width="50"
            > </el-table-column>
            <el-table-column
              prop="money"
              label="金额(元)"
            >
            <template slot-scope="scope">
              ￥{{ scope.row.money | numberCommas}}
            </template>
            </el-table-column>
            <el-table-column
              prop="userName"
              label="姓名"
            >
            </el-table-column>
            <el-table-column
              prop="createDate"
              label="创建时间"
              width="160"
            >
            </el-table-column>
            <el-table-column
              prop="departmentName"
              label="部门"
            > </el-table-column>
            <el-table-column
              prop="budgetName"
              label="费用预算类型"
              :show-overflow-tooltip="true"
            >
            </el-table-column>
            <!-- <el-table-column prop="expenseTypeName" label="费用类型" sortable="custom">
            </el-table-column> -->
            <el-table-column
              fixed="right"
              label="操作"
              width="100"
            >
              <template slot-scope="scope">
                <el-button
                  @click="handleClick(scope.row)"
                  type="text"
                  size="small"
                >详细</el-button>
              </template>
            </el-table-column>
          </el-table>
          <el-pagination background :total="pagination.total"
          :current-page="pagination.currentPage" layout="total, prev, pager, next"
          @current-change="pageChange" style="text-align:center;margin-top:15px;"></el-pagination>
        </div>
        <div
          slot="footer"
          class="dialog-footer"
          style="text-align:right;"
        >
          <el-button
            @click="closeDialog"
            type="primary"
          >关 闭</el-button>
        </div>
      </div>
    </el-dialog>

    <!-- 费用明细详情 -->
    <div v-if="ExpensesClaimFormVisible">
      <!-- title="费用明细" -->
      <el-dialog
        :visible.sync="ExpensesClaimFormVisible"
        :modal-append-to-body="true"
        :close-on-click-modal="false"
        custom-class="dialog-fullscreen"
        center
        @closed="closed"
        :before-close="closed"
        width="800px"
      >
        <ExpensesClaimForm
          :operaType="operaType"
          :id="expenseId"
        >
        </ExpensesClaimForm>
      </el-dialog>
    </div>
    <!-- 公司空间审核流程 -->
    <ExamineDialog v-if="ExpensesClaimFormVisible" ref="examineDialog" :visible.sync="ExpensesClaimFormVisible"
      :isExamine="isExamine" :operaType="operaType" :searchFlowType="searchFlowType" :flowId="flowId"
      :flowInstanceId="flowInstanceId" :jobTaskId="jobTaskId" :flowNodeType="flowNodeType" :initiatorId="initiatorId"
      :nextNodeProxyId="nextNodeProxyId" :flowProxyId="flowProxyId" :flowNodeProxyId="flowNodeProxyId"
      :actionType="actionType" :flowType="flowType" :flowInstanceBizRelevanceList="flowInstanceBizRelevanceList"/>
    <!-- 审核弹窗(对formMakiing制作的表单的审核) -->
    <EnterpriseExamineDialog v-if="examineDialogVisible" :visible.sync="examineDialogVisible" :btnVisible="false"
      :isExamine="isExamine" :flowId="flowId" :flowNodeType="flowNodeType" :nextNodeProxyId="nextNodeProxyId"
      :formId="formId" :flowNodeProxyId="flowNodeProxyId"
      :jobTaskId="jobTaskId" :initiatorId="initiatorId" :flowInstanceId="flowInstanceId" :businessId="businessId"
      :companyId="companyId" :selectFlowType="selectFlowType" :flowInstanceBizRelevanceList="flowInstanceBizRelevanceList"/>
    <!-- 公司空间审核流程结束 -->
  </div>
</template>

<script>
import ExpensesClaimForm from './ExpensesClaimForm.vue';
import Api from '@/api';
import ExamineDialog from '@/views/GroupApproveManage/components/ExamineDialog.vue';
import EnterpriseExamineDialog from '@/views/GroupApproveManage/components/EnterpriseExamineDialog.vue';
export default {
  name: 'DetailDialog',
  data() {
    return {
      query: { departmentId: '', budgetId: '', date: [] },
      costTypeList: [],
      tableData: [
      ],
      departmentList: [],
      ExpensesClaimFormVisible: false,
      operaType: '',
      expenseId: '',
      pagination: {
        total: 0,
        size: 10,
        currentPage: 1,
      },
      isExamine:false,
      searchFlowType:'',
      flowId:'',
      flowInstanceId:'',
      jobTaskId:'',
      flowNodeType:'',
      initiatorId:'',
      nextNodeProxyId:'',
      actionType:'',
      flowType:'',
      examineDialogVisible:false,
      formId:'',
      flowNodeProxyId:'',
      businessId:'',
      companyId:'',
      selectFlowType:'',
      flowInstanceBizRelevanceList:[]
    };
  },
  components: {
    ExpensesClaimForm,
    ExamineDialog,
    EnterpriseExamineDialog
  },
  props: {
    detailVisible: {
      type: Boolean,
      default: false
    },
    detailRow: {
      type: Object,
      default: function () {
        return {};
      }
    },
    //是否显示部门
    isShowDepartment:{
      type:Boolean,
      default:true
    },
    //是否显示归口
    isShowBudget:{
      type:Boolean,
      default:true
    },
    //外面传入的归口类型
    budgetId:{
      type:String,
      default:''
    }
  },
  filters:{
    numberCommas(x){
      if(x){
        var res = x.toString().replace(/\d+/, function (n) {
          return n.replace(/(\d)(?=(\d{3})+$)/g, function ($1) {
            return $1 + ",";
          });
        });
        return res;
      }else{
        return x
      }
    },
  },
  created() {
    this.query.departmentId = this.detailRow.departmentId;
    this.query.budgetId = this.detailRow.budgetTypeId
    // if(this.budgetId)this.query.budgetId = this.budgetId
    // this.getDepartmentList();
    this.getBudgetTypeOfGroup();
    this.getCostTypeList();
    this.getexpendList();
  },
  methods: {
    pageChange(val){
      this.pagination.currentPage = val
      this.getexpendList();
    },
    handleClick(row) {
      // console.log('row',row)
      // null:费用报销 expense_budget 1:差旅 travel_expense,2:请款 request_funds,3:借款 expense_loan,4:还款 expense_repayment
      let {type} = row.expenseReimbursement
      let flowType = ''
      let mainId = row.expenseReimbursement.id
      if(!type){
        flowType = 'expense_budget'
      }else if(type == 1){
        flowType = 'travel_expense'
      }else if(type == 2){
        flowType = 'request_funds'
      }else if(type == 3){
        flowType = 'expense_loan'
      }else if(type == 4){
        flowType = 'expense_repayment'
      }else if(type == 5){
        flowType = 'contract_payment_form'
        mainId = row.businessId
      }else if(type == 6){
        flowType = 'budget_adjustment'
        mainId = row.businessId || '-'
      }
      this.handleView(mainId,flowType)
      // this.operaType = 'check';
      // this.expenseId = row.expenseReimbursement.id;
      // this.ExpensesClaimFormVisible = true;
    },
    previewHandle(row){
      this.flowInstanceBizRelevanceList = row.flowInstanceBizRelevanceList ? row.flowInstanceBizRelevanceList.map(item => ({ ...item })) : [];
      if (!row.formExist) { // formMaking表单
        this.isExamine = false;
        this.lastCountersignFlag = row.lastCountersignFlag;// 判断是否为当前节点最后一个审批人--选择下一个分支节点
        this.initiatorId = row.initiatorId;
        this.btnVisible = false;
        this.flowId = row.flowProxyId;
        this.flowInstanceId = row.id;
        this.formId = row.formProxyId;
        this.flowNodeProxyId = row.flowNodeProxyId;
        this.flowNodeType = row.flowNextNodeAuditType;
        this.nextNodeProxyId = row.nextNodeProxyId;
        this.nextNodeName = row.nextNodeName;
        this.jobTaskId = row.jobTaskId;
        const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
        this.businessId = find?.otherBizId || '';
        const company = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'company');
        this.companyId = company?.otherBizId || '';

        this.examineDialogVisible = true;
      } else { // 固定页面
        this.operaType = 'edit'; // 费用预算用的操作
        this.actionType = 'examine'; // 其他无表单流程的操作
        if (row.auditWay == 'annual_perf') {
          const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'manageTarget');
          if (find) { // 管理指标
            this.flowType = 'manageTarget';
          } else { // 工作指标
            this.flowType = 'workTarget';
          }
        }
        this.searchFlowType = row.auditWay;
        this.isExamine = false;
        this.lastCountersignFlag = row.lastCountersignFlag;// 判断是否为当前节点最后一个审批人--选择下一个分支节点
        this.initiatorId = row.initiatorId;
        if (row.flowInstanceBizRelevanceList.length == 1) {
          this.flowId = row.flowInstanceBizRelevanceList[0].otherBizId; // 业务id，绑定的什么业务就是什么
        } else {
          const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
          this.flowId = find.otherBizId;
        }

        this.flowNodeProxyId = row.flowNodeProxyId;
        this.flowInstanceId = row.flowInstanceBizRelevanceList[0].flowInstanceId;
        this.flowNodeType = row.flowNextNodeAuditType;
        this.nextNodeProxyId = row.nextNodeProxyId;
        this.nextNodeName = row.nextNodeName;
        this.jobTaskId = row.jobTaskId;
        this.flowProxyId = row.flowProxyId;
        this.ExpensesClaimFormVisible = true;
      }
    },
    handleView(mainId,type) {
      //查询当前绑定的流程，调用查看弹窗
      this.getInstanceId(mainId,type).then(data=>{
        if(data){
          this.previewHandle(data)
        }
      })
    },
    // 获取流程实例id
    getInstanceId(id, type,taskStatus) {
      var otherBiz = type
      const flowInstanceBizRelevanceList = [{
        otherBiz,
        otherBizId: id,
      }];
      const data = {
        useScope: 'invest',
        initiator: 'all',
        flowInstanceBizRelevanceList
      };
      let api
      if(taskStatus == 'edit'){
        data.taskStatus = "waiting_send"
        api = Api.approveManage.getTaskList
      }else{
        api = Api.schedule.getFlowInstanceList
      }
      return new Promise((resolve, reject) => {
        this.$axios.post(api, { data, size: 1, pagination: true, pages: 1 }).then(res => {
          if (res.isSuccess) {
            let data = res?.data || []
            if (data.length) {
              resolve(data[0])
            } else {
              this.$message.error('数据已删除')
              resolve()
            }
          }
        });
      });
    },
    // 获取当前预算模板
    getBudgetTypeOfGroup() {
      this.$axios.post(
        Api.budgetManage.getBudgetCentralizedOfGroup,
        {},
        res => {
          if (res.isSuccess) {
            const data = res.data || [];
            const find = data.find(item => item.companyVo.id == this.detailRow.companyId);
            if (find) {
              this.centralizedApiVos = find.centralizedApiVos[0];
              this.projectBudgetCentralizedApiVos = find.projectBudgetCentralizedApiVos
              this.generateTree();
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    generateTree() {
      const { deptBudgetCentralizedVoList } = this.centralizedApiVos;
      const departOptions = []
      deptBudgetCentralizedVoList.forEach(item => {
        const { sysDepartmentVo } = item;
        departOptions.push({
          id:sysDepartmentVo.id,
          name:sysDepartmentVo.departmentName == '公司领导'?  '公司固定费用':sysDepartmentVo.departmentName,
          hasSelect: false
        })
      });
      this.projectBudgetCentralizedApiVos.forEach(item=>{
        departOptions.push({
          id:item.projectVo.id,
          name:item.projectVo.shortName || item.projectVo.name,
          hasSelect: false,
          isProject:true
        })
      })
      this.departmentList = departOptions
    },
    // 获取费用报销列表
    getexpendList() {
      this.$axios.post(
        Api.budgetManage.getexpendList,
        {
          startDate: this.query.date && this.query.date.length ? this.query.date[0] : '',
          endDate: this.query.date && this.query.date.length ? this.query.date[1] : '',
          status: 'end',
          expenseType:false,
          data: {
            // companyIds: this.detailRow.companyId ? [this.detailRow.companyId] : [],
            companyIds: [],
            //departmentIds: this.query.departmentId ? [this.query.departmentId] : [],
            expenseReimbursementIds: [],
            budgetId: this.query.budgetId
            // expenseReimbursementIds: this.query.budgetId ? [this.query.budgetId] : []
            // expenseReimbursementIds: [this.query.budgetId]
            // projectId:this.$store.state.user.projectId // 如果是项目中的需要传多一个项目id
          },
          pagination:true,
          pages:this.pagination.currentPage,
          size:this.pagination.size,
        },
        res => {
          if (res.isSuccess) {
            this.tableData = res.data ? res.data.dataList : [];
            this.pagination.total = res.data.total
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 费用类型列表(归口)
    getCostTypeList() {
      this.$axios.post(
        // Api.budgetManage.getCostTypeList,
        Api.budgetManage.getBudgetList,
        {
          data: {
            stringList: [
              '1','2'
            ],
            // annually: 2022,
            // annually: this.seletcTime,
            annually: new Date().getFullYear(),
            departmentId: this.query.departmentId,
            // companyId:this.detailRow.companyId
            // departmentId: this.detailRow.departmentId
          }
        },
        res => {
          if (res.isSuccess) {
            let dataList = res.data?.dataList || []
            this.costTypeList = dataList.filter(item=>item.companyId == this.detailRow.companyId)
            // this.costTypeList = res.data?.dataList//res.data ? res.data : [];
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    getSummaries(param) {
      const { columns, data } = param;
      const sums = [];
      columns.forEach((column, index) => {
        if (index === 0) {
          sums[index] = '总价';
          return;
        }
        const values = data.map(item => {
          if (item[column.property] == null) {
            return NaN;
          } else {
            return Number(item[column.property]);
          }
        });

        if (!values.every(value => isNaN(value))) {
          sums[index] = values.reduce((prev, curr) => {
            const value = Number(curr);
            if (!isNaN(value)) {
              return (prev * 100 + curr * 100) / 100;
            } else {
              return prev;
            }
          }, 0);
          sums[index] = this.fixedTwo(sums[index]);
          sums[index] += ' 元';
        } else {
          sums[index] = '';
        }
      });

      return sums;
    },
    fixedTwo(num) {
      var result = parseFloat(num);
      if (isNaN(result)) {
        return false;
      }
      result = Math.round(num * 100) / 100;
      return result;
    },
    selectDepartment() {
      this.query.budgetId = '';
      this.getCostTypeList();
      this.getexpendList();
    },
    selectExpendType() {
      this.getexpendList();
    },
    selectDate() {
      // console.log(this.query);
      this.getexpendList();
    },
    // 提交审批前先保存
    handleBeforeSubmit() {
      // const formRef = this.$refs.steps2.$refs.ExpensesClaimForm.$refs.infoForm;
      // formRef.validate((valid) => {
      //   if (valid) {
      //   } else {
      //     return false;
      //   }
      // });
      // const infoForm = this.$refs.steps2.$refs.ExpensesClaimForm.infoForm;
      // const param = {
      //   data: {
      //     projectId: infoForm.projectId,
      //     companyId: infoForm.companyId,
      //     attachmentCount: infoForm.attachmentCount,
      //     remark: infoForm.remark
      //   },
      //   expenseBudgetList: infoForm.expenseBudgetList.map(x => {
      //     return {
      //       budgetId: x.type.length > 1 ? x.type[x.type.length - 1] : '',
      // mainId: x.type[1],
      //       departmentId: x.type[0],
      //       money: x.money,
      //       type: infoForm.projectId ? 1 : 2 // 选了项目传1
      //     };
      //   }),
      //   taxInfoList: infoForm.taxInfoList,
      //   expenseDetailList: infoForm.expenseDetailList,
      //   expenseInAccountInfoList: infoForm.expenseInAccountInfoList
      // };
      // console.log(param);
      // this.$axios.post(
      //   '/web/expenseReimbursement/submit',
      //   param,
      //   (res) => {
      //     if (res.isSuccess) {
      //       // this.$message.success('提交成功！');
      //     } else {
      //       this.$message.error(res.message);
      //     }
      //   }

      // );
    },
    closed() {
      this.ExpensesClaimFormVisible = false;
    },
    closeDialog() {
      this.$emit('update:detailVisible', false);
    }
  }
};
</script>

<style>
</style>
