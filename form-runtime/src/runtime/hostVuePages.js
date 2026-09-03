import CompanyBudget from '@runtime/views/BudgetManage/CompanyBudget/components/addNewBudget.vue'
import EditCompanyBudget from '@runtime/views/BudgetManage/CompanyBudget/components/editeBudget.vue'
import CompanyBudgetAppend from '@runtime/views/BudgetManage/CompanyBudget/components/appendBueget.vue'
import CompanyMouthlyBudget from '@runtime/views/BudgetManage/MonthlyBudget/components/AddMonthlyBudget.vue'
import ProjectBudget from '@runtime/views/BudgetManage/ProjectBudget/components/NewBudget.vue'
import ExpensesClaimForm from '@runtime/views/BudgetManage/CompanyBudget/components/ExpensesClaimForm.vue'
import GroupFinance from '@runtime/views/BudgetManage/CompanyBudget/components/groupFinance.vue'
import CompanyMonthlyFinance from '@runtime/views/BudgetManage/CompanyBudget/components/companyMonthlyFinance.vue'
import AnnualPerformence from '@runtime/views/PerformanceManage/TargetBook/WorkTarget/index.vue'
import MonthlyPerformence from '@runtime/views/PerformanceManage/MonthlyPerf/MonthlyPerf.vue'
import MonthPerfSum from '@runtime/views/PerformanceManage/monthlyPerfBlocSum/monthPerfSum.vue'
import ReserveMonthSum from '@runtime/views/PerformanceManage/reserveMonthSum/reserveMonthSum.vue'
import AnnualAssessmentNoForm from '@runtime/views/PerformanceManage/staffAnnualAssessment/AnnualAssessmentNoForm.vue'
import TravelExpenseForm from '@runtime/views/GroupApproveManage/Submitted/components/Form/TravelExpenseForm.vue'
import PaymentBill from '@runtime/views/GroupApproveManage/Submitted/components/Form/PaymentBill.vue'
import LoadBill from '@runtime/views/GroupApproveManage/Submitted/components/Form/LoadBill.vue'
import ContractReview from '@runtime/components/NoFormFLow/ContractReview.vue'
import ContractPayRequest from '@runtime/components/NoFormFLow/ContractPayRequest.vue'
import BuyPlan from '@runtime/components/NoFormFLow/BuyPlan.vue'
import BuyDemand from '@runtime/components/NoFormFLow/BuyDemand.vue'
import BuyOrder from '@runtime/components/NoFormFLow/BuyOrder.vue'
import Invoice from '@runtime/components/NoFormFLow/Invoice.vue'

// HOST_VUE_PAGES 来自复制运行时的真实页面注册和 NoFormFLow 目录；这里只建立组件入口，不复制业务规则。
export const HOST_VUE_PAGES = {
  CompanyBudget,
  EditCompanyBudget,
  CompanyBudgetAppend,
  CompanyMouthlyBudget,
  EditCompanyMouthlyBudget: CompanyMouthlyBudget,
  ProjectBudget,
  ProjectBudgetAppend: ProjectBudget,
  ExpensesClaimForm,
  GroupFinance,
  companyMonthlyFinance: CompanyMonthlyFinance,
  AnnualPerformence,
  MonthlyPerformence,
  monthPerfSum: MonthPerfSum,
  reserveMonthSum: ReserveMonthSum,
  AnnualAssessmentNoForm,
  TravelExpenseForm,
  PaymentBill,
  LoadBill,
  ContractReview,
  ContractPayRequest,
  BuyPlan,
  BuyDemand,
  BuyOrder,
  Invoice
}

// resolveHostVuePage 只按运行时提供的组件名解析，不按流程名称猜测页面。
export function resolveHostVuePage (componentName) {
  return HOST_VUE_PAGES[String(componentName || '').trim()] || null
}
