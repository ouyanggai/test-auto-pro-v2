// 自动生成 - 由 sync.js 后处理
// stub 掉非流程组件，保留流程相关组件

import orgTree from './components/orgTree.vue';
import flowDetail from './components/flowDetail.vue';
import loanMoney from './components/loanMoney.vue';

// stub: commonAccounts (导航栏组件，流程不需要)
const commonAccounts = { render: h => h('div') };
// stub: invoiceCommonInfo (合同管理组件，流程不需要)
const invoiceCommonInfo = { render: h => h('div') };

export default { orgTree, commonAccounts, flowDetail, invoiceCommonInfo, loanMoney };
