import CompanyBudget from '@runtime/views/BudgetManage/CompanyBudget/components/addNewBudget.vue'
import EditCompanyBudget from '@runtime/views/BudgetManage/CompanyBudget/components/editeBudget.vue'
import CompanyBudgetAppend from '@runtime/views/BudgetManage/CompanyBudget/components/appendBueget.vue'
import CompanyMouthlyBudget from '@runtime/views/BudgetManage/MonthlyBudget/components/AddMonthlyBudget.vue'
import ProjectBudget from '@runtime/views/BudgetManage/ProjectBudget/components/NewBudget.vue'
import ExpensesClaimForm from '@runtime/views/BudgetManage/CompanyBudget/components/ExpensesClaimForm.vue'
import CompanyAmountAdjustForm from '@runtime/views/GroupApproveManage/Submitted/components/Form/CompanyAmountAdjustForm.vue'
import GroupFinance from '@runtime/views/BudgetManage/CompanyBudget/components/groupFinance.vue'
import CompanyMonthlyFinance from '@runtime/views/BudgetManage/CompanyBudget/components/companyMonthlyFinance.vue'
import AnnualPerformence from '@runtime/views/PerformanceManage/TargetBook/WorkTarget/index.vue'
import MonthlyPerformence from '@runtime/views/PerformanceManage/MonthlyPerf/MonthlyPerf.vue'
import MonthPerfSum from '@runtime/views/PerformanceManage/monthlyPerfBlocSum/monthPerfSum.vue'
import ReserveMonthSum from '@runtime/views/PerformanceManage/reserveMonthSum/reserveMonthSum.vue'
import AnnualAssessmentNoForm from '@runtime/views/PerformanceManage/staffAnnualAssessment/AnnualAssessmentNoForm.vue'
import ManagePerformence from '@runtime/views/PerformanceManage/TargetBook/ManageTarget/index.vue'
import TravelExpenseForm from '@runtime/views/GroupApproveManage/Submitted/components/Form/TravelExpenseForm.vue'
import PaymentBill from '@runtime/views/GroupApproveManage/Submitted/components/Form/PaymentBill.vue'
import LoadBill from '@runtime/views/GroupApproveManage/Submitted/components/Form/LoadBill.vue'
import ContractReview from '@runtime/components/NoFormFLow/ContractReview.vue'
import ContractPayRequest from '@runtime/components/NoFormFLow/ContractPayRequest.vue'
import BuyPlan from '@runtime/components/NoFormFLow/BuyPlan.vue'
import BuyDemand from '@runtime/components/NoFormFLow/BuyDemand.vue'
import BuyOrder from '@runtime/components/NoFormFLow/BuyOrder.vue'
import Invoice from '@runtime/components/NoFormFLow/Invoice.vue'
import HostNoFormPage from '../HostNoFormPage.vue'

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
  CompanyAmountAdjustForm,
  GroupFinance,
  companyMonthlyFinance: CompanyMonthlyFinance,
  AnnualPerformence,
  MonthlyPerformence,
  monthPerfSum: MonthPerfSum,
  reserveMonthSum: ReserveMonthSum,
  AnnualAssessmentNoForm,
  ManagePerformence,
  company_annual_budget: CompanyBudget,
  edit_company_annual_budget: EditCompanyBudget,
  add_annual_budget: CompanyBudgetAppend,
  depart_monthly_budget: CompanyMouthlyBudget,
  project_setup_budget: ProjectBudget,
  company_monthly_budget: CompanyMonthlyFinance,
  add_project_budget: ProjectBudget,
  expense_reimbursement: CompanyAmountAdjustForm,
  expense_budget: ExpensesClaimForm,
  group_finance: GroupFinance,
  annual_perf: AnnualPerformence,
  year_kpi_work_target: AnnualPerformence,
  staff_annual_assessment: AnnualAssessmentNoForm,
  monthly_perf: MonthlyPerformence,
  monthly_perf_companySum: MonthPerfSum,
  monthly_perf_departmentSum: MonthPerfSum,
  reserveMonthSum: ReserveMonthSum,
  travel_expense: TravelExpenseForm,
  request_funds: PaymentBill,
  loan: LoadBill,
  contract_review: ContractReview,
  contract_pay_request: ContractPayRequest,
  buy_plan: BuyPlan,
  buy_demand: BuyDemand,
  buy_order: BuyOrder,
  invoice: Invoice,
  invoice_apply: Invoice,
  NoFormFlow: HostNoFormPage,
  noform: HostNoFormPage,
  notform: HostNoFormPage,
  general_flow: HostNoFormPage,
  enterprise: HostNoFormPage,
  refund_bid: HostNoFormPage,
  wind_solar_condition: HostNoFormPage,
  TravelExpenseForm,
  PaymentBill,
  LoadBill,
  ContractReview,
  ContractPayRequest,
  BuyPlan,
  BuyDemand,
  BuyOrder,
  Invoice,
  HostNoFormPage
}

// resolveHostVuePage 只按运行时提供的组件名解析，不按流程名称猜测页面。
export function resolveHostVuePage (componentName) {
  const key = String(componentName || '').trim()
  // 自定义页面键可能来自目标配置而未进入复制运行时注册表；通用页仍需呈现快照，不能让工作区退化为空白。
  return HOST_VUE_PAGES[key] || HOST_VUE_PAGES[key.toLowerCase()] || HOST_VUE_PAGES.NoFormFlow
}
