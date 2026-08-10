<!--
 * @Descripttion: 步骤2
 * @Author: zhengzetao
 * @Date: 2022-06-15
-->
<template>
  <div style="height: 100%;">
    <!-- 公共关联流程 -->
    <FormCommonFlowPost ref="flowListSelect"/>
    <ExpensesClaimForm v-if="selectFlowType == 'expense_budget'" :selectFlowName="selectFlowName" :operaType="'add'"
      :flowProxyId="flowId" ref="ExpensesClaimForm" />
    <template v-else>
      <component v-bind:is="currentComponent"
              :ref="currentRef"
              @changeComponent="changeComponent"
              :paramsInfo="params"
              :param="param"
              :annual="annual"
              :operaType="operaType"
              :showType="showType"
              :selectFlowType="selectFlowType"
              :flowProxyId="flowId"
              :actionType="actionType"
              ></component>
    </template>
    <CheckFlowNodeDetail v-if="checkViewFlowDetailVisible" :dialogVisible.sync="checkViewFlowDetailVisible" :initiatorId="initiatorId"
    :flowId="flowId" :isDraft="true"></CheckFlowNodeDetail>
  </div>
</template>

      <!-- 费用报销单 -->
      <!-- <ExpensesClaimForm :operaType="operaType" :id="flowId" :flowProxyId="flowProxyId"
        :flowNodeProxyId="flowNodeProxyId" v-if="searchFlowType == 'expense_budget'" ref="ExpensesClaimForm">
      </ExpensesClaimForm> -->
      <!-- 金额调剂 -->
      <!-- <CompanyAmountAdjustForm :operaType="operaType" :id="flowId" v-if="searchFlowType == 'expense_reimbursement'"
        ref="CompanyAmountAdjustForm">
      </CompanyAmountAdjustForm> -->
<script>
import CheckFlowNodeDetail from '../../components/CheckFlowNodeDetail.vue';
import Api from '@/api';
import CompanyBudget from '@/views/BudgetManage/CompanyBudget/components/addNewBudget.vue';
import EditCompanyBudget from '@/views/BudgetManage/CompanyBudget/components/editeBudget.vue';
import CompanyBudgetAppend from '@/views/BudgetManage/CompanyBudget/components/appendBueget.vue';
import CompanyMouthlyBudget from '@/views/BudgetManage/MonthlyBudget/components/AddMonthlyBudget.vue';
import EditCompanyMouthlyBudget from '@/views/BudgetManage/MonthlyBudget/components/AddMonthlyBudget.vue';
import ProjectBudget from '@/views/BudgetManage/ProjectBudget/components/NewBudget.vue';
// import EditProjectBudget from '@/views/BudgetManage/ProjectBudget/components/NewBudget.vue'
import ProjectBudgetAppend from '@/views/BudgetManage/ProjectBudget/components/NewBudget.vue';
import ExpensesClaimForm from '@/views/BudgetManage/CompanyBudget/components/ExpensesClaimForm.vue';
import CompanyAmountAdjustForm from '@/views/GroupApproveManage/Submitted/components/Form/CompanyAmountAdjustForm.vue';
// import AnnualPerformence from '@/views/GroupApproveManage/Submitted/components/Form/AnnualPerformence.vue';
import AnnualPerformence from '@/views/PerformanceManage/TargetBook/WorkTarget/index';
import ManagePerformence from '@/views/PerformanceManage/TargetBook/ManageTarget/index';
import MonthlyPerformence from '@/views/PerformanceManage/MonthlyPerf/MonthlyPerf.vue';
import monthPerfSum from '@/views/PerformanceManage/monthlyPerfBlocSum/monthPerfSum.vue';
import reserveMonthSum from '@/views/PerformanceManage/reserveMonthSum/reserveMonthSum.vue';
import GroupFinance from '@/views/BudgetManage/CompanyBudget/components/groupFinance.vue';
import companyMonthlyFinance from '@/views/BudgetManage/CompanyBudget/components/companyMonthlyFinance.vue'
import AnnualAssessmentNoForm from '@/views/PerformanceManage/staffAnnualAssessment/AnnualAssessmentNoForm.vue'
const FormCommonFlowPost = () => import('@/views/GroupApproveManage/components/FormCommonFlowPost/index.vue');
// import EditCompanyMouthlyBudget from '@/views/BudgetManage/CompanyBudget/components/appendBueget.vue'
// import { importAll } from "@/utils";

// const allFile = importAll(
//   require.context("@/views/GroupApproveManage/Submitted/components/Form/", true, /\.vue$/)
// );

// const componentsAll = {};
// for (const key in allFile) {
//   const element = allFile[key].default;
//   componentsAll[element.name] = element;
// }
export default {
  name: 'Steps2',
  components: {
    CheckFlowNodeDetail,
    CompanyBudget,
    EditCompanyBudget,
    CompanyBudgetAppend,
    CompanyMouthlyBudget,
    ProjectBudget,
    EditCompanyMouthlyBudget,
    // EditProjectBudget,
    ProjectBudgetAppend,
    ExpensesClaimForm,
    CompanyAmountAdjustForm,
    AnnualPerformence,
    ManagePerformence,
    MonthlyPerformence,
    monthPerfSum,
    reserveMonthSum,
    GroupFinance,
    companyMonthlyFinance,
    FormCommonFlowPost,
    AnnualAssessmentNoForm
    // ...componentsAll
  },
  props: {
    selectFlowType: {
      type: String,
      default: ''
    },
    selectFlowName: {
      type: String,
      default: ''
    },
    flowId: {
      type: String,
      default: ''
    },
    flowType: {
      type: String,
      default: ''
    }
  },
  data() {
    return {
      checkViewFlowDetailVisible: false,
      selectFlowTypeName: this.selectFlowType,
      params: {},
      annual: '',
      operaType: '',
      param: {},
      showType: '',
      pageOperaType: '',
      actionType: 'create'
    };
  },
  computed: {
    currentComponent() {
      console.log('this.selectFlowTypeName',this.selectFlowTypeName)
      if (this.selectFlowType == 'annual_perf'||this.selectFlowType == 'year_kpi_work_target') {
        if (this.flowType == 'workTarget') { // 跳转到AnnualPerformence这个组件
          return this.$store.state.settings.budgetPagesName[this.selectFlowTypeName].component;
        } else {
          return 'ManagePerformence'; // 管理指标
        }
      } else {
        return this.$store.state.settings.budgetPagesName[this.selectFlowTypeName].component;
      }
    },
    currentRef() {
      return this.selectFlowTypeName;
    }
  },
  watch: {
    flowId: {
      handler(val) {
        if (val) {
          this.getFlowFindById();
        }
      },
      immediate: true
    }
  },
  created() {
    if (this.$route?.query?.isExamine) {
      this.actionType = 'edit';
    }
  },
  mounted() {
    console.log('sssssss', this.flowId,this.selectFlowType);
  },
  methods: {
    handleCheckFlow() { //
      console.log('handleCheckFlow',22)
      this.initiatorId = this.$store.state.user.userId;
      this.checkViewFlowDetailVisible = true;
    },
    // 换一个表单
    changeComponent(data) {
      this.selectFlowTypeName = data.type;
      if (data.type == 'company_annual_budget') { // 新建预算
        this.annual = data.annual || data.list.annual;
      } else if (data.type == 'edit_depart_monthly_budget') {
        this.param = data.list;
        this.operaType = true;
        this.showType = 'edit';
      }
      // else if(data.type == 'add_project_budget'){
      //   this.param = data.list
      //   this.showType = 'edit'
      // }else if(data.type == 'add_project_budget_edit'){
      //   this.selectFlowTypeName = 'add_project_budget'
      //   this.param = data.list
      //   this.showType = 'append'
      // }
      else {
        this.showType = 'init';
        this.params = data.list;
      }
    },
    // 判断无表单流程的下一节点是否并行或手动
    getFlowFindById() {
      this.$axios.post(
        Api.schedule.flowTemplateFindById,
        {
          data: {
            id: this.flowId
          }
        },
        (res) => {
          if (res.isSuccess) {
            let formTemplateBizRelevanceVoList = res?.data?.formTemplateBizRelevanceVoList
            if(formTemplateBizRelevanceVoList.find(el=>el.otherBiz == 'company')){
              this.$emit('update:flowProjectId','')
            }
            if (res.data.flowNodeTemplate.childFlowNodeTemplate.type == 'parallel') {
              // 处理发起人后并行节点审批人自选
              const parallelChooseNodes = [];
              let hasChoose = false;
              res.data.flowNodeTemplate.childFlowNodeTemplate.parallelNodes.forEach(parallelNode => {
                if (parallelNode.childFlowNodeTemplate.flowNodeAuditConfig.auditType == 'run_node_choose') {
                  hasChoose = true;
                  parallelChooseNodes.push(
                    {
                      nodeName: parallelNode.childFlowNodeTemplate.nodeName,
                      nextNodeTemplateId: parallelNode.nextNodeTemplateId,
                      nodeAuditList: []
                    }
                  );
                }
              });
              if (hasChoose) {
                this.$emit('update:parallelChooseNodes', parallelChooseNodes);
              }
            }
            if (res.data.flowNodeTemplate.childFlowNodeTemplate.branchExecuteType == 'custom_choose') {
              // 处理发起人后的手动分支选择
              const manualChooseNodes = [];
              res.data.flowNodeTemplate.childFlowNodeTemplate.conditionNodes.forEach(branch => {
                manualChooseNodes.push(
                  {
                    nextNodeTemplateId: branch.nextNodeTemplateId,
                    nodeName: branch.childFlowNodeTemplate.nodeName,
                    nodeType: branch.childFlowNodeTemplate.type, // 为处理手动分支
                    branchName: branch.name,
                    auditType: branch.childFlowNodeTemplate.flowNodeAuditConfig.auditType
                  }
                );
              });
              this.$emit('update:manualChooseNodes', manualChooseNodes);
            }
          }
        }
      );
    }
  }
};
</script>

<style lang="scss">
  ::v-deep .fm-form .fm-report-table__table .fm-report-table__td {
    padding: 4px !important;
  }
</style>
