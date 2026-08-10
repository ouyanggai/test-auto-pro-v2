import Vue from 'vue'
import ElementUI from 'element-ui'
import 'element-ui/lib/theme-chalk/index.css'

import FormMaking from '@vendor/FormMaking.common'
import '@vendor/FormMaking.css'
import '@/assets/styles/formMaking.scss'

import App from './App.vue'
import targetComponents from './runtime/targetComponents'

Vue.config.productionTip = false
Vue.use(ElementUI, { size: 'small' })
// 保持目标副本的自定义组件注册名；组件若依赖外部宿主，由模板扫描明确标为 unsupported，不能降级成普通控件。
Vue.use(FormMaking, { lang: 'zh-CN', components: targetComponents })

new Vue({ render: (createElement) => createElement(App) }).$mount('#app')
