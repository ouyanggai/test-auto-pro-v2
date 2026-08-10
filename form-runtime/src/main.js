import App from './App.vue'
import '@runtime/main.js'

// 完整 rsh-flow-components 入口先注册真实 FormMaking、自定义组件、Vuex 与原生路由；本地只追加隔离的配置路由。
window.$router.addRoutes([{ path: '/test-auto-form', name: 'testAutoFormRuntime', component: App }])
window.$router.replace('/test-auto-form').catch(() => {})
