import Vue from 'vue';
import App from './App.vue';
import router from './router';
import store from './store';

// 样式
import 'normalize.css/normalize.css';
import 'element-ui/lib/theme-chalk/index.css';
import '@/assets/styles/index.scss';

// Element UI
import ElementUI, { Message } from 'element-ui';
Vue.use(ElementUI, { size: 'mini' });

// FormMaking + 自定义组件（路径与源项目一致）
import FormMaking from '@/lib/vue-form-making';
import '@/lib/vue-form-making/dist/FormMaking.css';

import CustomeUploadExcel from '@/components/Custom/components/CustomeUploadExcel';
import OutBoundMaterialSelect from '@/components/Custom/components/InventoryMaterialSelect/outBound.vue';
import InBoundMaterialSelect from '@/components/Custom/components/InventoryMaterialSelect/inBound.vue';
import CustomeWeather from '@/components/Custom/components/CustomeWeather';
import CustomeSelectProject from '@/components/Custom/components/CustomeSelectProject/index.vue';
import ExpenseBudgetType from '@/components/Custom/components/ExpenseBudgetType/index.vue';
import GeneralListSelectShow from '@/components/Custom/components/GeneralListSelectShow/index.vue';
import ContractChangeListSelect from '@/components/Custom/components/ContractChangeListSelect.vue';
import PersonMulSelect from '@/components/Custom/components/PersonMulSelect/index.vue';
import GeneralFlowListMulSelect from '@/components/Custom/components/GeneralFlowListMulSelect/index.vue';
import CustomeInfoSelect from '@/components/Custom/components/CustomeInfoSelect/index.vue';
import LtdOrDepSelect from '@/components/Custom/components/LtdOrDepSelect/index.vue';
import CustomeFileView from '@/components/Custom/components/CustomeFileView';
import CustomeFileImport from '@/components/Custom/components/CustomeFileImport/index.vue';
import LegalContractDocTable from '@/components/Custom/components/Business/LegalContractDocTable';
import ContractSealReviewBusiness from '@/components/Custom/components/Business/ContractSealReviewBusiness';
import FlowListMulSelect from '@/components/Custom/components/FlowListMulSelect/index.vue';
import RequestPayout from '@/components/Custom/components/RequestPayout/index.vue';
import CitySelect from '@/components/Custom/components/CitySelect/index.vue';
import TravelRoutePlanning from '@/components/Custom/components/TravelRoutePlanning/index.vue';
import TravelOrderManagement from '@/components/Custom/components/TravelOrderManagement/index.vue';
import SupplierEntry from '@/components/Custom/components/Supplier/SupplierEntry.vue';
import SupplierAccess from '@/components/Custom/components/Supplier/SupplierAccess.vue';
import SupplierClear from '@/components/Custom/components/Supplier/SupplierClear.vue';
import SupplierReEntry from '@/components/Custom/components/Supplier/SupplierReEntry.vue';
import SupplierChange from '@/components/Custom/components/Supplier/SupplierChange.vue';

Vue.use(FormMaking, {
  lang: 'zh-CN',
  components: [
    {
      // excel文件数据获取
      name: 'custom-upload-excel',
      component: CustomeUploadExcel
    },
    {
      // 出库材料选择
      name: 'out-bound-material-select',
      component: OutBoundMaterialSelect
    },
    {
      // 入库材料选择
      name: 'in-bound-material-select',
      component: InBoundMaterialSelect
    },
    {
      // excel文件数据获取
      name: 'custom-upload-excel',
      component: CustomeUploadExcel
    },
    {
      // 天气
      name: 'custom-weather',
      component: CustomeWeather
    },
    {
      // 项目选择组件
      name: 'custome-select-project',
      component: CustomeSelectProject
    },
    {
      // 费用预算类型选择组件
      name: 'custome-expense-budgetType',
      component: ExpenseBudgetType
    },
    {
      // 通用列表选择
      name: 'general-list-select-show',
      component: GeneralListSelectShow
    },
    {
      // 合同变更专用合同选择
      name: 'contract-change-list-select',
      component: ContractChangeListSelect
    },
    {
      // 人员多选
      name: 'person-mulSelect',
      component: PersonMulSelect
    },
    {
      // 通用流程列表多选
      name: 'general-flow-list-mulSelect',
      component: GeneralFlowListMulSelect
    },
    {
      // 通用信息组件
      name: 'custome-info-select',
      component: CustomeInfoSelect
    },
    {
      // 选择公司或部门（单选或多选）
      name: 'ltd-or-dep-select',
      component: LtdOrDepSelect
    },
    {
      // 文件预览与下载
      name: 'custome-file-view',
      component: CustomeFileView
    },
    {
      // 导入模板
      name: 'custome-file-import',
      component: CustomeFileImport
    },
    {
      // 合同合规性评审文件表格
      name: 'legal-contract-doctable',
      component: LegalContractDocTable
    },
    {
      // 合同盖章评审业务
      name: 'contract-seal-review-business',
      component: ContractSealReviewBusiness
    },
    { // 流程(多选)
      name: 'flow-list-mul-select',
      component: FlowListMulSelect
    },
    { // 请款单
      name: 'request_payout',
      component: RequestPayout
    },
    {
      // 城市选择
      name: 'city-select',
      component: CitySelect
    },
    {
      // 行程规划
      name: 'travel-route-planning',
      component: TravelRoutePlanning
    },
    {
      // 差旅订单管理
      name: 'travel-order-management',
      component: TravelOrderManagement
    },
    {
      // 供应商入库申请
      name: 'supplier-entry',
      component: SupplierEntry
    },
    {
      // 供应商年度综合评估
      name: 'supplier-access',
      component: SupplierAccess
    },
    {
      // 供应商清退
      name: 'supplier-clear',
      component: SupplierClear
    },
    {
      // 黑名单供应商重新入库
      name: 'supplier-re-entry',
      component: SupplierReEntry
    },
    {
      // 供应商变更
      name: 'supplier-change',
      component: SupplierChange
    }
  ]
});

// Print
import Print from 'vue-print-nb';
Vue.use(Print);

// Side-effect imports
import './utils/directives.js';
import './utils/secret';

// SVG icons
import '@/assets/icons';

// Filters
import * as filters from '@/filters';
Object.keys(filters).forEach(key => {
  Vue.filter(key, filters[key]);
});

// Directives
import '@/directives';

// axios
import axios from '@/utils/axios';
Vue.prototype.$axios = axios;
Vue.prototype.$message = Message;

// bus
Vue.prototype.$bus = new Vue();

// fmCustomPage 全局注册（源项目在 layout/index.vue 中注册）
import fmCustomPage from '@/components/fmComponents/index.js';
Vue.use(fmCustomPage);

// $initFlow / $flowDetail
import '@/components/InitFlowDialog';

// postMessage 适配器
import { initPostMessage } from '@/adapters/postMessage';
import { setupRuntimeAuthSync } from '@/utils/runtimeAuthSync';
import { installBackendRequestRouting } from '@/utils/backendRequestRouting';
import { baseUrl } from '@/config/env';

Vue.config.productionTip = false;

setupRuntimeAuthSync(store);
if (process.env.VUE_APP_FLAG === 'test') {
  installBackendRequestRouting(window, { apiBaseUrl: baseUrl });
}
initPostMessage();

const app = new Vue({
  router,
  store,
  render: h => h(App)
}).$mount('#app');

// 全局引用
window.$router = router;
window.$store = store;
window.$vue = app;
