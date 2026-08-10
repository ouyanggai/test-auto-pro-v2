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
import FormMaking from '@/lib/vue-form-making/dist/FormMaking.common';
import '@/lib/vue-form-making/dist/FormMaking.css';
import CustomeUploadExcel from '@/components/Custom/components/CustomeUploadExcel';
import CustomeWeather from '@/components/Custom/components/CustomeWeather';
import ExpenseBudgetType from '@/components/Custom/components/ExpenseBudgetType/index.vue';
import CustomeSelectProject from '@/components/Custom/components/CustomeSelectProject/index.vue';
import CustomeInfoSelect from '@/components/Custom/components/CustomeInfoSelect/index.vue';
import LtdOrDepSelect from '@/components/Custom/components/LtdOrDepSelect/index.vue';
import FlowListMulSelect from '@/components/Custom/components/FlowListMulSelect/index.vue';
import RequestPayout from '@/components/Custom/components/RequestPayout/index.vue';
import GeneralListSelectShow from '@/components/Custom/components/GeneralListSelectShow/index.vue';
import GeneralFlowListMulSelect from '@/components/Custom/components/GeneralFlowListMulSelect/index.vue';
import PersonMulSelect from '@/components/Custom/components/PersonMulSelect/index.vue';
import CustomeFileView from '@/components/Custom/components/CustomeFileView';
import CustomeFileImport from '@/components/Custom/components/CustomeFileImport/index.vue';
import LegalContractDocTable from '@/components/Custom/components/Business/LegalContractDocTable';
import ContractSealReviewBusiness from '@/components/Custom/components/Business/ContractSealReviewBusiness';
import OutBoundMaterialSelect from '@/components/Custom/components/InventoryMaterialSelect/outBound.vue';
import InBoundMaterialSelect from '@/components/Custom/components/InventoryMaterialSelect/inBound.vue';
import CitySelect from '@/components/Custom/components/CitySelect/index.vue';
import TravelRoutePlanning from '@/components/Custom/components/TravelRoutePlanning/index.vue';
import TravelOrderManagement from '@/components/Custom/components/TravelOrderManagement/index.vue';

Vue.use(FormMaking, {
  lang: 'zh-CN',
  components: [
    { name: 'custom-upload-excel', component: CustomeUploadExcel },
    { name: 'out-bound-material-select', component: OutBoundMaterialSelect },
    { name: 'in-bound-material-select', component: InBoundMaterialSelect },
    { name: 'custom-weather', component: CustomeWeather },
    { name: 'custome-select-project', component: CustomeSelectProject },
    { name: 'custome-expense-budgetType', component: ExpenseBudgetType },
    { name: 'general-list-select-show', component: GeneralListSelectShow },
    { name: 'person-mulSelect', component: PersonMulSelect },
    { name: 'general-flow-list-mulSelect', component: GeneralFlowListMulSelect },
    { name: 'custome-info-select', component: CustomeInfoSelect },
    { name: 'ltd-or-dep-select', component: LtdOrDepSelect },
    { name: 'custome-file-view', component: CustomeFileView },
    { name: 'custome-file-import', component: CustomeFileImport },
    { name: 'legal-contract-doctable', component: LegalContractDocTable },
    { name: 'contract-seal-review-business', component: ContractSealReviewBusiness },
    { name: 'flow-list-mul-select', component: FlowListMulSelect },
    { name: 'request_payout', component: RequestPayout },
    { name: 'city-select', component: CitySelect },
    { name: 'travel-route-planning', component: TravelRoutePlanning },
    { name: 'travel-order-management', component: TravelOrderManagement }
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

Vue.config.productionTip = false;

setupRuntimeAuthSync(store);
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
