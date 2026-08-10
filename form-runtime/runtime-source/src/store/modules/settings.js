/*
 * @Descripttion:
 * @Author: Calvin
 * @Date: 2021-01-14 14:22:24
 */
import defaultSettings from '@/settings';

const { showSettings, fixedHeader, sidebarLogo } = defaultSettings;

const state = {
  showSettings: showSettings,
  fixedHeader: fixedHeader,
  sidebarLogo: sidebarLogo,
  budgetPagesName: { // 预算流程的无表单页面名称和映射的组件名称
    company_annual_budget: {
      name: '公司年度预算',
      component: 'CompanyBudget',
      operaType: 'init',
      isShow: true
    },
    edit_company_annual_budget: {
      name: '编辑年度预算',
      component: 'EditCompanyBudget',
      operaType: 'edit',
      isShow: false
    },
    add_annual_budget: {
      name: '追加公司年度预算',
      component: 'CompanyBudgetAppend',
      operaType: 'append',
      isShow: true
    },
    depart_monthly_budget: {
      name: '部门月度预算',
      component: 'CompanyMouthlyBudget',
      operaType: 'init',
      isShow: true
    },
    // 'edit_depart_monthly_budget': {
    //   name: '编辑公司月度预算',
    //   component: 'EditCompanyMouthlyBudget',
    //   operaType:'edit'
    // },
    project_setup_budget: {
      name: '项目立项预算',
      component: 'ProjectBudget',
      operaType: 'init',
      isShow: true
    },
    company_monthly_budget: {
      name: '公司月度预算',
      component: 'companyMonthlyFinance',
    },
    add_project_budget: {
      name: '追加项目预算',
      component: 'ProjectBudgetAppend',
      operaType: 'append',
      isShow: true
    },
    travel_expense: {
      name: '差旅报销',
      component: 'TravelExpenseForm',
      isShow: false
    },
    request_funds: {
      name: '请款单',
      component: 'PaymentBill',
      isShow: false
    },
    loan: {
      name: '借款单',
      component: 'LoadBill',
      isShow: false
    },
    expense_budget: {
      name: '费用报销',
      component: 'ExpensesClaimForm',
      isShow: true
    },
    group_finance: {
      name: '集团各公司年度预算',
      component: 'GroupFinance',
      isShow: true
    },
    annual_perf: {
      name: '年度绩效',
      component: 'AnnualPerformence',
      isShow: true
    },
    staff_annual_assessment: {
      name: '员工年终考核',
      component: 'AnnualAssessmentNoForm',
      isShow: true
    },
    year_kpi_work_target: {
      name: '年度目标责任书',
      component: 'AnnualPerformence',
      isShow: true
    },
    monthly_perf: {
      name: '月度绩效',
      component: 'MonthlyPerformence',
      isShow: true
    },
    monthly_perf_companySum: {
      name: '公司月度绩效汇总',
      component: 'monthPerfSum',
      isShow: true
    },
    monthly_perf_departmentSum: {
      name: '部门月度绩效汇总',
      component: 'monthPerfSum',
      isShow: true
    },
    reserveMonthSum: {
      name: '后备英才月度绩效汇总',
      component: 'reserveMonthSum',
      isShow: true
    },
    contract_review: {
      name: '合同评审',
      isShow: true
    },
    contract_pay_request: {
      name: '合同付款申请',
      isShow: true
    },
    invoice_apply: {
      name: '开票申请',
      isShow: true
    },
    request_funds: {
      name: '请款',
      isShow: true
    },
    refund_bid: {
      name: '投标保证金退还请款',
      isShow: true
    },
    wind_solar_condition: {
      name: '风光项目测算条件',
      isShow: true
    },
    general_flow: {
      name: '普通流程',
      component: '',
      isShow: true
    },
    enterprise: {
      name: '其他类型',
      component: '',
      isShow: true
    }
  },
  contractPagesName: {
    buy_plan: '招标/采购计划审批表',
    contract_review: '合同评审表',
    contract_pay_request: '合同付款申请表',
    contract_pay: '合同支付表',
    buy_demand: '采购需求申请表',
    buy_order: '采购单',
    invoice: '发货单'
  }
};

const mutations = {
  CHANGE_SETTING: (state, { key, value }) => {
    // eslint-disable-next-line no-prototype-builtins
    if (state.hasOwnProperty(key)) {
      state[key] = value;
    }
  }
};

const actions = {
  changeSetting({ commit }, data) {
    commit('CHANGE_SETTING', data);
  }
};

export default {
  namespaced: true,
  state,
  mutations,
  actions
};

