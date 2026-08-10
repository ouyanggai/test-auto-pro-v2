import Vue from 'vue'
import ElementUI from 'element-ui'
import 'element-ui/lib/theme-chalk/index.css'

import FormMaking from '@/lib/vue-form-making/dist/FormMaking.common'
import '@/lib/vue-form-making/dist/FormMaking.css'
import '@/assets/styles/formMaking.scss'

import App from './App.vue'

Vue.config.productionTip = false
Vue.use(ElementUI, { size: 'small' })
Vue.use(FormMaking, { lang: 'zh-CN', components: [] })

new Vue({ render: (createElement) => createElement(App) }).$mount('#app')
